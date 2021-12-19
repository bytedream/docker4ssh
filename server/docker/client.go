package docker

import (
	"docker4ssh/database"
	"github.com/docker/docker/client"
)

type Client struct {
	Client   *client.Client
	Database *database.Database
	Network  Network
}
