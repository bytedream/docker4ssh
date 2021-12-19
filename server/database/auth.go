package database

import (
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	User     *string `json:"user"`
	Password *[]byte `json:"password"`
}

func NewAuth(user string, password []byte) (Auth, error) {
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return Auth{}, err
	}
	return Auth{
		&user,
		&hash,
	}, nil
}

func NewUnsafeAuth(user string, password []byte) Auth {
	auth, _ := NewAuth(user, password)
	return auth
}

func (db *Database) SetAuth(containerID string, auth Auth) error {
	if auth.User != nil {
		_, err := db.Exec("INSERT INTO auth (container_id, user) VALUES ($1, $2) ON CONFLICT (container_id) DO UPDATE SET user=$2", containerID, *auth.User)
		if err != nil {
			return err
		}
	}
	if auth.Password != nil {
		_, err := db.Exec("INSERT INTO auth (container_id, password) VALUES ($1, $2) ON CONFLICT (container_id) DO UPDATE SET password=$2", containerID, *auth.Password)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetAuthByContainer returns the auth by a container id
func (db *Database) GetAuthByContainer(containerID string) (auth Auth, exists bool) {
	if err := db.QueryRow("SELECT user, password FROM auth WHERE container_id=$1", containerID).Scan(&auth.User, &auth.Password); err != nil {
		return Auth{}, false
	}
	return auth, true
}

func (db *Database) GetContainerByAuth(auth Auth) (containerID string, exists bool) {
	// return true if `auth` contains a nil pointer or no auth was found in the database.
	// hopefully this is no security issue
	if auth.User == nil || auth.Password == nil {
		return "", false
	}
	if err := db.QueryRow("SELECT container_id FROM auth WHERE user=$1 AND password=$2 OR password IS NULL", auth.User, auth.Password).Scan(&containerID); err != nil {
		return "", false
	}
	return containerID, true
}

func (db *Database) DeleteAuth(containerID string) error {
	_, err := db.Exec("DELETE FROM auth WHERE container_id=$1", containerID)
	return err
}
