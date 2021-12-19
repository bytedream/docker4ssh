package api

import (
	"docker4ssh/database"
	"docker4ssh/ssh"
	"encoding/json"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

type authGetResponse struct {
	User        string `json:"user"`
	HasPassword bool   `json:"has_password"`
}

func AuthGet(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	auth, ok := database.GetDatabase().GetAuthByContainer(user.Container.FullContainerID)

	if ok {
		return authGetResponse{
			User:        *auth.User,
			HasPassword: auth.Password != nil,
		}, http.StatusOK
	} else {
		return APIError{Message: "no auth is set"}, http.StatusNotFound
	}
}

type authPostRequest struct {
	User     *string `json:"user"`
	Password *string `json:"password"`
}

func AuthPost(w http.ResponseWriter, r *http.Request, user *ssh.User) (interface{}, int) {
	var request authPostRequest
	json.NewDecoder(r.Body).Decode(&request)
	defer r.Body.Close()

	db := database.GetDatabase()

	auth, _ := db.GetAuthByContainer(user.Container.FullContainerID)

	if request.User != nil {
		if *request.User == "" {
			return APIError{Message: "new username cannot be empty"}, http.StatusNotAcceptable
		}
		if err := db.SetAuth(user.Container.FullContainerID, database.Auth{
			User: request.User,
		}); err != nil {
			zap.S().Errorf("Error while updating user for user %s: %v", user.ID, err)
			return APIError{Message: "failed to process user"}, http.StatusInternalServerError
		}
		zap.S().Infof("Updated password for %s", user.Container.ContainerID)
	}
	if request.Password != nil && *request.Password == "" {
		if err := db.DeleteAuth(user.Container.FullContainerID); err != nil {
			zap.S().Errorf("Error while deleting auth for user %s: %v", user.ID, err)
			return APIError{Message: "failed to delete auth"}, http.StatusInternalServerError
		}
		zap.S().Infof("Deleted authenticiation for %s", user.Container.ContainerID)
	} else if request.Password != nil {
		pwd, err := bcrypt.GenerateFromPassword([]byte(*request.Password), bcrypt.DefaultCost)
		if err != nil {
			zap.S().Errorf("Error while updating password for user %s: %v", user.ID, err)
			return APIError{Message: "failed to process password"}, http.StatusInternalServerError
		}
		var username string
		if auth.User == nil {
			username = user.Container.FullContainerID
		} else {
			username = *auth.User
		}
		if err = db.SetAuth(user.Container.FullContainerID, database.NewUnsafeAuth(username, pwd)); err != nil {
			return APIError{Message: "failed to update authentication"}, http.StatusInternalServerError
		}
		zap.S().Infof("Updated password for %s", user.Container.ContainerID)
	}
	return nil, http.StatusOK
}
