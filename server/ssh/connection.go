package ssh

import (
	"bytes"
	"context"
	"database/sql"
	"docker4ssh/database"
	"docker4ssh/docker"
	"docker4ssh/utils"
	"fmt"
	"go.uber.org/zap"
	"strconv"
	"sync"
	"time"
)

var (
	allContainers []*docker.InteractiveContainer
)

func closeAllContainers(ctx context.Context) {
	var wg sync.WaitGroup
	for _, container := range allContainers {
		wg.Add(1)
		container := container
		go func() {
			container.Stop(ctx)
			wg.Done()
		}()
	}
	wg.Wait()
}

func connection(client *docker.Client, user *User) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	container, ok := getContainer(ctx, client, user)
	if !ok {
		zap.S().Errorf("Failed to create container for %s", user.ID)
		return
	}

	user.Container = container.SimpleContainer

	var found bool
	for _, cont := range allContainers {
		if cont == container {
			found = true
		}
	}
	if !found {
		allContainers = append(allContainers, container)
	}

	// check if the container is running and start it if not
	if running, err := container.Running(ctx); err == nil && !running {
		if err = container.Start(ctx); err != nil {
			zap.S().Errorf("Failed to start container %s: %v", container.ContainerID, err)
			fmt.Fprintln(user.Terminal, "Failed to start container")
			return
		}
		zap.S().Infof("Started container %s with internal id '%s', ip '%s'", container.ContainerID, container.ContainerID, container.Network.IP)
	} else if err != nil {
		zap.S().Errorf("Failed to get container running state: %v", err)
		fmt.Fprintln(user.Terminal, "Failed to check container running state")
	}

	config := container.Config()
	if user.Profile.StartupInformation {
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "┌───Container────────────────┐\r\n")
		fmt.Fprintf(buf, "│ Container ID: %-12s │\r\n", container.ContainerID)
		fmt.Fprintf(buf, "│ Network Mode: %-12s │\r\n", config.NetworkMode.Name())
		fmt.Fprintf(buf, "│ Configurable: %-12t │\r\n", config.Configurable)
		fmt.Fprintf(buf, "│ Run Level:    %-12s │\r\n", config.RunLevel.Name())
		fmt.Fprintf(buf, "│ Exit After:   %-12s │\r\n", config.ExitAfter)
		fmt.Fprintf(buf, "│ Keep On Exit: %-12t │\r\n", config.KeepOnExit)
		fmt.Fprintf(buf, "└──────────────Information───┘\r\n")
		user.Terminal.Write(buf.Bytes())
	}

	// start a new terminal session
	if err := container.Terminal(ctx, user.Terminal); err != nil {
		zap.S().Errorf("Failed to serve %s terminal: %v", container.ContainerID, err)
		fmt.Fprintln(user.Terminal, "Failed to serve terminal")
	}

	if config.RunLevel == docker.User && container.TerminalCount() == 0 {
		if err := container.Stop(ctx); err != nil {
			zap.S().Errorf("Error occoured while stopping container %s: %v", container.ContainerID, err)
		} else {
			lenBefore := len(allContainers)
			for i, cont := range allContainers {
				if cont == container {
					allContainers[i] = allContainers[lenBefore-1]
					allContainers = allContainers[:lenBefore-1]
					break
				}
			}
			if lenBefore == len(allContainers) {
				zap.S().Warnf("Stopped container %s, but failed to remove it from the global container scope", container.ContainerID)
			} else {
				zap.S().Infof("Stopped container %s", container.ContainerID)
			}
		}
	}

	zap.S().Infof("Stopped session for user %s", user.ID)
}

func getContainer(ctx context.Context, client *docker.Client, user *User) (container *docker.InteractiveContainer, ok bool) {
	db := database.GetDatabase()
	var config docker.Config

	// check if the user has a container (id) assigned
	if user.Profile.ContainerID != "" {
		for _, cont := range allContainers {
			if cont.FullContainerID == user.Profile.ContainerID {
				return cont, true
			}
		}

		settings, err := db.SettingsByContainerID(user.Profile.ContainerID)
		if err != nil {
			zap.S().Errorf("Failed to get stored container config for container %s: %v", user.Profile.ContainerID, err)
			fmt.Fprintf(user.Terminal, "Could not connect to saved container")
			return nil, false
		}

		config = docker.Config{
			NetworkMode:        docker.NetworkMode(*settings.NetworkMode),
			Configurable:       *settings.Configurable,
			RunLevel:           docker.RunLevel(*settings.RunLevel),
			StartupInformation: *settings.StartupInformation,
			ExitAfter:          *settings.ExitAfter,
			KeepOnExit:         *settings.KeepOnExit,
		}

		container, err = docker.InteractiveContainerFromID(ctx, client, config, user.Profile.ContainerID)
		if err != nil {
			zap.S().Errorf("Failed to get container from id %s: %v", user.Profile.ContainerID, err)
			fmt.Fprintf(user.Terminal, "Failed to get container")
			return nil, false
		}

		zap.S().Infof("Re-used container %s for user %s", user.Profile.ContainerID, user.ID)
	} else {
		config = docker.Config{
			NetworkMode:        docker.NetworkMode(user.Profile.NetworkMode),
			Configurable:       user.Profile.Configurable,
			RunLevel:           docker.RunLevel(user.Profile.RunLevel),
			StartupInformation: user.Profile.StartupInformation,
			ExitAfter:          user.Profile.ExitAfter,
			KeepOnExit:         user.Profile.KeepOnExit,
		}

		image, out, err := docker.NewImage(ctx, client.Client, user.Profile.Image)
		if err != nil {
			zap.S().Errorf("Failed to get '%s' image for profile %s: %v", user.Profile.Image, user.Profile.Name(), err)
			fmt.Fprintf(user.Terminal, "Failed to get image %s", image.Ref())
			return nil, false
		}
		if out != nil {
			if err := utils.DisplayJSONMessagesStream(out, user.Terminal, user.Terminal); err != nil {
				zap.S().Fatalf("Failed to fetch '%s' docker image: %v", image.Ref(), err)
				fmt.Fprintf(user.Terminal, "Failed to fetch image %s", image.Ref())
				return nil, false
			}
		}

		container, err = docker.NewInteractiveContainer(ctx, client, config, image, strconv.Itoa(int(time.Now().Unix())))
		if err != nil {
			zap.S().Errorf("Failed to create interactive container: %v", err)
			fmt.Fprintln(user.Terminal, "Failed to create interactive container")
			return nil, false
		}

		zap.S().Infof("Created new %s container (%s) for user %s", image.Ref(), container.ContainerID, user.ID)
	}

	if _, err := db.SettingsByContainerID(container.FullContainerID); err != nil {
		if err == sql.ErrNoRows {
			rawNetworkMode := int(config.NetworkMode)
			rawRunLevel := int(config.RunLevel)
			if err := db.SetSettings(container.FullContainerID, database.Settings{
				NetworkMode:        &rawNetworkMode,
				Configurable:       &config.Configurable,
				RunLevel:           &rawRunLevel,
				StartupInformation: &config.StartupInformation,
				ExitAfter:          &config.ExitAfter,
				KeepOnExit:         &config.KeepOnExit,
			}); err != nil {
				zap.S().Errorf("Failed to update settings for container %s for user %s: %v", container.ContainerID, user.ID, err)
				return nil, false
			}
		}
	}

	return container, true
}
