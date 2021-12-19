package api

import (
	"docker4ssh/ssh"
	"net/http"
	"time"
)

type pingGetResponse struct {
	Received int64 `json:"received"`
}

func PingGet(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	return pingGetResponse{Received: time.Now().UnixNano()}, http.StatusOK
}
