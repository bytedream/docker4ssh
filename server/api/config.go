package api

import (
	"context"
	"docker4ssh/docker"
	"docker4ssh/ssh"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"reflect"
	"strings"
)

type configGetResponse struct {
	NetworkMode        docker.NetworkMode `json:"network_mode"`
	Configurable       bool               `json:"configurable"`
	RunLevel           docker.RunLevel    `json:"run_level"`
	StartupInformation bool               `json:"startup_information"`
	ExitAfter          string             `json:"exit_after"`
	KeepOnExit         bool               `json:"keep_on_exit"`
}

func ConfigGet(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	config := user.Container.Config()

	return configGetResponse{
		config.NetworkMode,
		config.Configurable,
		config.RunLevel,
		config.StartupInformation,
		config.ExitAfter,
		config.KeepOnExit,
	}, http.StatusOK
}

type configPostRequest configGetResponse

var configPostRequestLookup, _ = structJsonLookup(configPostRequest{})

type configPostResponse struct {
	Message  string                       `json:"message"`
	Rejected []configPostResponseRejected `json:"rejected"`
}

type configPostResponseRejected struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func ConfigPost(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	var requestBody map[string]interface{}
	json.NewDecoder(r.Body).Decode(&requestBody)
	defer r.Body.Close()

	var change bool
	var response configPostResponse

	updatedConfig := user.Container.Config()

	for k, v := range requestBody {
		if v == nil {
			continue
		}

		kind, ok := configPostRequestLookup[k]
		if !ok {
			response.Rejected = append(response.Rejected, configPostResponseRejected{
				Name:        k,
				Description: fmt.Sprintf("name / field %s does not exist", k),
			})
		} else {
			valueKind := reflect.TypeOf(v).Kind()
			if valueKind != kind && valueKind == reflect.Float64 && kind == reflect.Int {
				valueKind = reflect.Int
			}

			if valueKind != kind {
				response.Rejected = append(response.Rejected, configPostResponseRejected{
					Name:        k,
					Description: fmt.Sprintf("value should be type %s, got type %s", kind, valueKind),
				})
			}

			change = true
			switch k {
			case "network_mode":
				updatedConfig.NetworkMode = docker.NetworkMode(v.(float64))
			case "configurable":
				updatedConfig.Configurable = v.(bool)
			case "run_level":
				updatedConfig.RunLevel = docker.RunLevel(v.(float64))
			case "startup_information":
				updatedConfig.StartupInformation = v.(bool)
			case "exit_after":
				updatedConfig.ExitAfter = v.(string)
			case "keep_on_exit":
				updatedConfig.KeepOnExit = v.(bool)
			}
		}
	}

	if len(response.Rejected) > 0 {
		var arr []string
		for _, rejected := range response.Rejected {
			arr = append(arr, rejected.Name)
		}

		if len(response.Rejected) == 1 {
			response.Message = fmt.Sprintf("1 invalid configuration was found: %s", strings.Join(arr, ", "))
			return response, http.StatusNotAcceptable
		} else if len(response.Rejected) > 1 {
			response.Message = fmt.Sprintf("%d invalid configurations were found: %s", len(response.Rejected), strings.Join(arr, ", "))
			return response, http.StatusNotAcceptable
		}
	} else if change {
		if err := user.Container.UpdateConfig(context.Background(), updatedConfig); err != nil {
			zap.S().Errorf("Error while updating config for API user %s: %v", user.ID, err)
			response.Message = "Internal error while updating the config"
			return response, http.StatusInternalServerError
		}
	}
	return nil, http.StatusOK
}
