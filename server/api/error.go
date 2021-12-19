package api

import (
	"docker4ssh/ssh"
	"net/http"
)

type errorGetResponse APIError

func ErrorGet(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	return APIError{Message: "Example error message"}, http.StatusBadRequest
}
