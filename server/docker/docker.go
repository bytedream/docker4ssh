package docker

import (
	"github.com/docker/docker/client"
	"os"
)

type NetworkMode int

const (
	Off NetworkMode = iota + 1

	// Isolate isolates the container from the host and the host's
	// network. Therefore, no configurations can be changed from
	// within the container
	Isolate

	// Host is the default docker networking configuration
	Host

	// Docker is the same configuration you get when you start a
	// container via the command line
	Docker

	// None disables all isolation between the docker container
	// and the host, so inside the network the container can act
	// as the host. So it has access to the host's network directly
	None
)

func (nm NetworkMode) Name() string {
	switch nm {
	case Off:
		return "Off"
	case Isolate:
		return "Iso"
	case Host:
		return "Host"
	case Docker:
		return "Docker"
	case None:
		return "None"
	}
	return "invalid network"
}

func (nm NetworkMode) NetworkName() string {
	switch nm {
	case Off:
		return "none"
	case Isolate:
		return "docker4ssh-full"
	case Host:
		return "bridge"
	case Docker:
		return "docker4ssh-def"
	case None:
		return "none"
	}
	return ""
}

type RunLevel int

const (
	User RunLevel = iota + 1
	Container
	Forever
)

func (rl RunLevel) Name() string {
	switch rl {
	case User:
		return "User"
	case Container:
		return "Container"
	case Forever:
		return "Forever"
	}
	return ""
}

type Config struct {
	// NetworkMode describes the level of isolation of the container to the host system.
	// Mostly changes the network of the container, see NetworkNames for more details
	NetworkMode NetworkMode

	// If Configurable is true, the container can change settings for itself
	Configurable bool

	// RunLevel describes the container behavior.
	// If the RunLevel is User, the container will exit when the user disconnects.
	// If the RunLevel is Container, the container keeps running if the user disconnects
	// and ExitAfter is specified and the specified process has not finished yet.
	// If the RunLevel is Forever, the container keeps running forever unless ExitAfter
	// is specified and the specified process ends.
	//
	// Note: It also automatically exits if ExitAfter is specified and the specified
	// process ends, even if the user is still connected to the container
	RunLevel RunLevel

	// StartupInformation defines if information about the container like its (shorthand)
	// container id, NetworkMode, RunLevel, etc. should be shown when connecting to it
	StartupInformation bool

	// ExitAfter contains a process name after which end the container should stop
	ExitAfter string

	// When KeepOnExit is true, the container won't get deleted if it stops working
	KeepOnExit bool
}

func InitCli() (*client.Client, error) {
	return client.NewClientWithOpts()
}

func IsRunning() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return !os.IsNotExist(err)
}
