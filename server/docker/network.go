package docker

import (
	"context"
	c "docker4ssh/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type Network map[NetworkMode]string

// InitNetwork initializes a new docker4ssh network
func InitNetwork(ctx context.Context, cli *client.Client, config *c.Config) (Network, error) {
	n := Network{}

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	for _, dockerNetwork := range networks {
		var mode NetworkMode

		switch dockerNetwork.Name {
		case "none":
			mode = Off
		case "docker4ssh-iso":
			mode = Isolate
		case "bridge":
			mode = Host
		case "docker4ssh-def":
			mode = Docker
		case "host":
			mode = None
		default:
			continue
		}

		n[mode] = dockerNetwork.ID
	}

	if _, ok := n[Isolate]; !ok {
		// create a new network which isolates the container from the host,
		// but not from the network
		resp, err := cli.NetworkCreate(ctx, "docker4ssh-iso", types.NetworkCreate{
			CheckDuplicate: true,
			Driver:         "bridge",
			EnableIPv6:     false,
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{
					{
						Subnet: config.Network.Isolate.Subnet,
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		n[Isolate] = resp.ID
	}

	if _, ok := n[Docker]; !ok {
		// the standard network for all containers
		resp, err := cli.NetworkCreate(ctx, "docker4ssh-def", types.NetworkCreate{
			CheckDuplicate: true,
			Driver:         "bridge",
			EnableIPv6:     false,
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{
					{
						Subnet: config.Network.Default.Subnet,
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		n[Docker] = resp.ID
	}

	return n, nil
}
