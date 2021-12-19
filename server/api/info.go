package api

import (
	"docker4ssh/ssh"
	"net/http"
)

type infoGetResponse struct {
	ContainerID string `json:"container_id"`
}

func InfoGet(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	return infoGetResponse{
		ContainerID: user.Container.FullContainerID,
	}, http.StatusOK
}
