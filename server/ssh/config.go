package ssh

import (
	c "docker4ssh/config"
	"docker4ssh/database"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

func NewSSHConfig(config *c.Config) (*ssh.ServerConfig, error) {
	db := database.GetDatabase()

	sshConfig := &ssh.ServerConfig{
		MaxAuthTries: 3,
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if containerID, exists := db.GetContainerByAuth(database.NewUnsafeAuth(conn.User(), password)); exists && containerID != "" {
				return &ssh.Permissions{
					CriticalOptions: map[string]string{
						"containerID": containerID,
					},
				}, nil
			} else if profile, ok := profiles.Match(conn.User(), password); ok {
				return &ssh.Permissions{
					CriticalOptions: map[string]string{
						"profile": profile.Name(),
					},
				}, nil
			} else if config.Profile.Dynamic.Enable && dynamicProfile.Match(conn.User(), password) {
				return &ssh.Permissions{
					CriticalOptions: map[string]string{
						"profile": "dynamic",
						"image":   conn.User(),
					},
				}, nil
			}
			// i think logging the wrong password is a bit unsafe.
			// if you have e.g. just a type in it isn't very well to see your nearly correct password in the logs
			return nil, fmt.Errorf("%s tried to connect with user %s but entered wrong a password", conn.RemoteAddr().String(), conn.User())
		},
	}
	sshConfig.SetDefaults()

	key, err := parseSSHPrivateKey(config.SSH.Keyfile, []byte(config.SSH.Passphrase))
	if err != nil {
		return nil, err
	}
	sshConfig.AddHostKey(key)

	return sshConfig, nil
}

func parseSSHPrivateKey(path string, password []byte) (ssh.Signer, error) {
	keyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var key ssh.Signer
	if len(password) == 0 {
		key, err = ssh.ParsePrivateKey(keyBytes)
	} else {
		key, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, password)
	}
	if err != nil {
		return nil, err
	}
	return key, nil
}
