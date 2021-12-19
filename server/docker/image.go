package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"io"
)

type Image struct {
	ref string
}

func (i Image) Ref() string {
	return i.ref
}

// NewImage creates a new Image instance
func NewImage(ctx context.Context, cli *client.Client, ref string) (Image, io.ReadCloser, error) {
	summary, err := cli.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", ref)),
	})
	if err != nil {
		return Image{}, nil, err
	}

	if len(summary) > 0 {
		return Image{
			ref: ref,
		}, nil, nil
	} else {
		out, err := cli.ImagePull(ctx, ref, types.ImagePullOptions{})
		if err != nil {
			return Image{}, nil, err
		}
		return Image{
			ref: ref,
		}, out, nil
	}
}
