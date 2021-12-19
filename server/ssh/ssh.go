package ssh

import (
	"context"
	"crypto/md5"
	c "docker4ssh/config"
	"docker4ssh/database"
	"docker4ssh/docker"
	"docker4ssh/terminal"
	"encoding/hex"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"net"
	"regexp"
	"strings"
)

var (
	users = make([]*User, 0)

	profiles       c.Profiles
	dynamicProfile c.Profile
)

type User struct {
	*ssh.ServerConn

	ID        string
	IP        string
	Profile   *c.Profile
	Terminal  *terminal.Terminal
	Container *docker.SimpleContainer
}

func GetUser(ip string) *User {
	for _, user := range users {
		if container := user.Container; container != nil && container.Network.IP == ip {
			return user
		}
	}
	return nil
}

type extras struct {
	containerID string
}

func StartServing(config *c.Config, serverConfig *ssh.ServerConfig) (errChan chan error, closer func() error) {
	errChan = make(chan error, 1)

	var err error
	profiles, err = c.LoadProfileDir(config.Profile.Dir, c.DefaultPreProfileFromConfig(config))
	if err != nil {
		errChan <- err
		return
	}
	zap.S().Debugf("Loaded %d profile(s)", len(profiles))

	if config.Profile.Dynamic.Enable {
		dynamicProfile, err = c.DynamicProfileFromConfig(config, c.DefaultPreProfileFromConfig(config))
		if err != nil {
			errChan <- err
			return
		}
		zap.S().Debugf("Loaded dynamic profile")
	}

	cli, err := docker.InitCli()
	if err != nil {
		errChan <- err
		return
	}
	zap.S().Debugf("Initialized docker cli")

	network, err := docker.InitNetwork(context.Background(), cli, config)
	if err != nil {
		errChan <- err
		return
	}
	zap.S().Debugf("Initialized docker networks")

	client := &docker.Client{
		Client:   cli,
		Database: database.GetDatabase(),
		Network:  network,
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.SSH.Port))
	if err != nil {
		errChan <- err
		return
	}
	zap.S().Debugf("Created ssh listener")

	var closed bool
	go func() {
		db := database.GetDatabase()

		for {
			conn, err := listener.Accept()
			if err != nil {
				if closed {
					return
				}
				zap.S().Errorf("Failed to accept new ssh user: %v", err)
				continue
			}
			serverConn, chans, requests, err := ssh.NewServerConn(conn, serverConfig)
			if err != nil {
				zap.S().Errorf("Failed to establish new ssh connection: %v", err)
				continue
			}

			idBytes := md5.Sum([]byte(strings.Split(serverConn.User(), ":")[0]))
			idString := hex.EncodeToString(idBytes[:])

			zap.S().Infof("New ssh connection from %s with %s (%s)", serverConn.RemoteAddr().String(), serverConn.ClientVersion(), idString)

			var profile *c.Profile
			if name, ok := serverConn.Permissions.CriticalOptions["profile"]; ok {
				if name == "dynamic" {
					if image, ok := serverConn.Permissions.CriticalOptions["image"]; ok {
						tempDynamicProfile := dynamicProfile
						tempDynamicProfile.Image = image
						profile = &tempDynamicProfile
					}
				}
				if profile == nil {
					if profile, ok = profiles.GetByName(name); !ok {
						zap.S().Errorf("Failed to get profile %s", name)
						continue
					}
				}
			} else if containerID, ok := serverConn.Permissions.CriticalOptions["containerID"]; ok {
				if settings, err := db.SettingsByContainerID(containerID); err == nil {
					profile = &c.Profile{
						NetworkMode:        *settings.NetworkMode,
						Configurable:       *settings.Configurable,
						RunLevel:           *settings.RunLevel,
						StartupInformation: *settings.StartupInformation,
						ExitAfter:          *settings.ExitAfter,
						KeepOnExit:         *settings.KeepOnExit,
						ContainerID:        containerID,
					}
				} else {
					for _, container := range allContainers {
						if container.ContainerID == containerID {
							cconfig := c.GetConfig()
							profile = &c.Profile{
								Password:           regexp.MustCompile(cconfig.Profile.Default.Password),
								NetworkMode:        cconfig.Profile.Default.NetworkMode,
								Configurable:       cconfig.Profile.Default.Configurable,
								RunLevel:           cconfig.Profile.Default.RunLevel,
								StartupInformation: cconfig.Profile.Default.StartupInformation,
								ExitAfter:          cconfig.Profile.Default.ExitAfter,
								KeepOnExit:         cconfig.Profile.Default.KeepOnExit,
								Image:              "",
								ContainerID:        containerID,
							}
						}
					}
				}
			}

			zap.S().Debugf("User %s has profile %s", idString, profile.Name())

			user := &User{
				ServerConn: serverConn,
				ID:         idString,
				Terminal:   &terminal.Terminal{},
				Profile:    profile,
			}
			users = append(users, user)

			go ssh.DiscardRequests(requests)
			go handleChannels(chans, client, user)
		}
	}()

	return errChan, func() error {
		closed = true

		// close all containers
		closeAllContainers(context.Background())

		// close the listener
		return listener.Close()
	}
}
