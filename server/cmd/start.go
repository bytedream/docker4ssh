package cmd

import (
	"docker4ssh/api"
	c "docker4ssh/config"
	"docker4ssh/database"
	"docker4ssh/docker"
	"docker4ssh/logging"
	"docker4ssh/ssh"
	"docker4ssh/validate"
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the docker4ssh server",
	Args:  cobra.MaximumNArgs(0),

	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preStart()
	},
	Run: func(cmd *cobra.Command, args []string) {
		start()
	},
}

func preStart() error {
	if !docker.IsRunning() {
		return fmt.Errorf("docker daemon is not running")
	}

	cli, err := docker.InitCli()
	if err != nil {
		return err
	}

	config, err := c.InitConfig(true)
	if err != nil {
		return err
	}

	validator := validate.NewConfigValidator(cli, false, config)

	if result := validator.ValidateLogging(); !result.Ok() {
		return fmt.Errorf(result.String())
	}

	level := zap.NewAtomicLevel()
	level.UnmarshalText([]byte(config.Logging.Level))
	var outputFiles, errorFiles []string
	if config.Logging.ConsoleOutput {
		outputFiles = append(outputFiles, "/dev/stdout")
	}
	if config.Logging.OutputFile != "" {
		outputFiles = append(outputFiles, config.Logging.OutputFile)
	}
	if config.Logging.ConsoleError {
		errorFiles = append(errorFiles, "/dev/stderr")
	}
	if config.Logging.ErrorFile != "" {
		errorFiles = append(errorFiles, config.Logging.ErrorFile)
	}
	logging.InitLogging(level, outputFiles, errorFiles)

	if result := validator.Validate(); !result.Ok() {
		return fmt.Errorf(result.String())
	}
	c.SetConfig(config)

	db, err := database.NewSqlite3Connection(config.Database.Sqlite3File)
	if err != nil {
		zap.S().Fatalf("Failed to initialize database: %v", err)
	}
	database.SetDatabase(db)

	return nil
}

func start() {
	config := c.GetConfig()

	if config.SSH.Passphrase == "" {
		zap.S().Warn("YOU HAVE AN EMPTY PASSPHRASE WHICH IS INSECURE, SUGGESTING CREATING A NEW SSH KEY WITH A PASSPHRASE.\n" +
			"IF YOU'RE DOWNLOADED THIS VERSION FROM THE RELEASES (https://github.com/ByteDream/docker4ssh/releases/latest), MAKE SURE TO CHANGE YOUR SSH KEY IMMEDIATELY BECAUSE ANYONE COULD DECRYPT THE SSH SESSION!!\n" +
			"USE 'ssh-keygen -t ed25519 -f /etc/docker4ssh/docker4ssh.key -b 4096' AND UPDATE THE PASSPHRASE IN /etc/docker4ssh/docker4ssh.conf UNDER ssh.Passphrase")
	}

	serverConfig, err := ssh.NewSSHConfig(config)
	if err != nil {
		zap.S().Fatalf("Failed to initialize ssh server config: %v", err)
	}

	sshErrChan, sshCloser := ssh.StartServing(config, serverConfig)
	zap.S().Infof("Started ssh serving on port %d", config.SSH.Port)
	apiErrChan, apiCloser := api.ServeAPI(config)
	zap.S().Infof("Started api serving on port %d", config.Api.Port)

	done := make(chan struct{})
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGUSR1, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig

		if sshCloser != nil {
			sshCloser()
		}
		if apiCloser != nil {
			apiCloser()
		}

		database.GetDatabase().Close()

		if s != syscall.SIGUSR1 {
			// Errorf is called here instead of Fatalf because the original exit signal should be kept to exit with it later
			zap.S().Errorf("(FATAL actually) received abort signal %d: %s", s.(syscall.Signal), strings.ToUpper(s.String()))
			os.Exit(int(s.(syscall.Signal)))
		}

		done <- struct{}{}
	}()

	select {
	case err = <-sshErrChan:
	case err = <-apiErrChan:
	}

	if err != nil {
		zap.S().Errorf("Failed to start working: %v", err)
		sig <- os.Interrupt
	} else {
		select {
		case <-sig:
			if err != nil {
				zap.S().Errorf("Serving failed due error: %v", err)
			} else {
				zap.S().Info("Serving stopped")
			}
		default:
			sig <- syscall.SIGUSR1
		}
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// if the timeout of 5 seconds expires, forcefully exit
		os.Exit(int(syscall.SIGKILL))
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
}
