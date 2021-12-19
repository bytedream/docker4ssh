package validate

import (
	"context"
	"docker4ssh/config"
	"docker4ssh/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func NewProfileValidator(cli *client.Client, strict bool, profile *config.Profile) *ProfileValidator {
	return &ProfileValidator{
		Validator: &Validator{
			Cli:    cli,
			Strict: strict,
		},
		Profile: profile,
	}
}

type ProfileValidator struct {
	*Validator

	Profile *config.Profile
}

func (pv *ProfileValidator) Validate() *ValidatorResult {
	profile := pv.Profile
	errors := make([]*ValidateError, 0)

	networkMode := docker.NetworkMode(profile.NetworkMode)
	if docker.Off > networkMode || networkMode > docker.None {
		errors = append(errors, newValidateError(profile.Name(), "NetworkMode", profile.NetworkMode, "not a valid network mode", nil))
	}
	runLevel := docker.RunLevel(profile.RunLevel)
	if docker.User > runLevel || runLevel > docker.Forever {
		errors = append(errors, newValidateError(profile.Name(), "RunLevel", profile.RunLevel, "is not a valid run level", nil))
	}
	if profile.Image == "" && profile.ContainerID == "" {
		errors = append(errors, newValidateError(profile.Name(), "image/container", "", "Image OR Container must be specified, neither both nor none", nil))
	} else if pv.Strict {
		if profile.Image != "" {
			list, err := pv.Cli.ImageList(context.Background(), types.ImageListOptions{
				Filters: filters.NewArgs(filters.Arg("reference", profile.Image)),
			})
			if err != nil || len(list) == 0 {
				errors = append(errors, newValidateError(profile.Name(), "Image", profile.Image, "image does not exist", nil))
			}
		} else if profile.ContainerID != "" {
			list, err := pv.Cli.ContainerList(context.Background(), types.ContainerListOptions{
				Filters: filters.NewArgs(filters.Arg("id", profile.ContainerID)),
			})
			if err != nil || len(list) == 0 {
				errors = append(errors, newValidateError(profile.Name(), "Image", profile.Image, "container does not exist", nil))
			}
		}
	}

	return &ValidatorResult{
		Strict: pv.Strict,
		Errors: errors,
	}
}
