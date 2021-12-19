package api

import (
	"bytes"
	"docker4ssh/config"
	"docker4ssh/ssh"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

type EndpointHandler struct {
	http.Handler

	auth bool

	get  func(http.ResponseWriter, *http.Request, *ssh.User) (interface{}, int)
	post func(http.ResponseWriter, *http.Request, *ssh.User) (interface{}, int)
}

func (h *EndpointHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := strings.Split(r.RemoteAddr, ":")[0]

	zap.S().Infof("User connected to api with remote address %s", ip)

	w.Header().Add("Content-Type", "application/json")

	user := ssh.GetUser(ip)
	// checks if auth should be checked and if so and no user could be found, response an error
	if h.auth && user == nil {
		zap.S().Errorf("Could not find api user with ip %s", ip)
		json.NewEncoder(w).Encode(APIError{Message: "unauthorized"})
		return
	} else if user != nil {
		zap.S().Debugf("API ip %s is %s", ip, user.ID)
	}

	raw := bytes.Buffer{}
	if r.ContentLength > 0 {
		io.Copy(&raw, r.Body)
		defer r.Body.Close()
		if !json.Valid(raw.Bytes()) {
			zap.S().Errorf("API user %s sent invalid body", ip)
			w.WriteHeader(http.StatusNotAcceptable)
			json.NewEncoder(w).Encode(APIError{Message: "invalid body"})
			return
		}
		r.Body = ioutil.NopCloser(&raw)
	}

	zap.S().Debugf("API user %s request - \"%s %s %s\" \"%s\" \"%s\"", ip, r.Method, r.URL.Path, r.Proto, r.UserAgent(), raw.String())

	var response interface{}
	var code int

	switch r.Method {
	case http.MethodGet:
		if h.get != nil {
			response, code = h.get(w, r, user)
		}
	case http.MethodPost:
		if h.post != nil {
			response, code = h.post(w, r, user)
		}
	}

	if response == nil && code == 0 {
		zap.S().Infof("API user %s sent invalid method: %s", ip, r.Method)
		response = APIError{Message: fmt.Sprintf("invalid method '%s'", r.Method)}
		code = http.StatusConflict
	} else {
		zap.S().Infof("API user %s issued %s successfully", ip, r.URL.Path)
	}

	w.WriteHeader(code)
	if response != nil {
		json.NewEncoder(w).Encode(response)
	}
}

func ServeAPI(config *config.Config) (errChan chan error, closer func() error) {
	errChan = make(chan error, 1)

	mux := http.NewServeMux()

	mux.Handle("/ping", &EndpointHandler{
		get: PingGet,
	})
	mux.Handle("/error", &EndpointHandler{
		get: ErrorGet,
	})
	mux.Handle("/info", &EndpointHandler{
		get:  InfoGet,
		auth: true,
	})
	mux.Handle("/config", &EndpointHandler{
		get:  ConfigGet,
		post: ConfigPost,
		auth: true,
	})
	mux.Handle("/auth", &EndpointHandler{
		get:  AuthGet,
		post: AuthPost,
		auth: true,
	})

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Api.Port))
	if err != nil {
		errChan <- err
		return
	}

	go func() {
		errChan <- http.Serve(listener, mux)
	}()

	return errChan, listener.Close
}
