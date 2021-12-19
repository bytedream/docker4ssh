package ssh

import (
	"docker4ssh/docker"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

type RequestType string

const (
	RequestPtyReq       RequestType = "pty-req"
	RequestWindowChange RequestType = "window-change"
)

type PtyReqPayload struct {
	Term string

	Width, Height           uint32
	PixelWidth, PixelHeight uint32

	Modes []byte
}

func handleChannels(chans <-chan ssh.NewChannel, client *docker.Client, user *User) {
	for channel := range chans {
		go handleChannel(channel, client, user)
	}
}

func handleChannel(channel ssh.NewChannel, client *docker.Client, user *User) {
	if t := channel.ChannelType(); t != "session" {
		channel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	conn, requests, err := channel.Accept()
	if err != nil {
		zap.S().Warnf("Failed to accept channel for user %s", user.ID)
		return
	}
	defer conn.Close()
	user.Terminal.ReadWriter = conn

	// handle all other request besides the normal user input.
	// currently, only 'pty-req' is implemented which determines a terminal size change
	go handleRequest(requests, user)

	// this handles the actual user terminal connection.
	// blocks until the session has finished
	connection(client, user)

	zap.S().Debugf("Session for user %s ended", user.ID)
}

func handleRequest(requests <-chan *ssh.Request, user *User) {
	for request := range requests {
		switch RequestType(request.Type) {
		case RequestPtyReq:
			// this could spam the logs when the user resizes his window constantly
			// log()

			var ptyReq PtyReqPayload
			ssh.Unmarshal(request.Payload, &ptyReq)

			user.Terminal.Width = ptyReq.Width
			user.Terminal.Height = ptyReq.Height
		case RequestWindowChange:
			// prevent from logging
		default:
			zap.S().Debugf("New request from user %s - Type: %s, Want Reply: %t, Payload: '%s'", user.ID, request.Type, request.WantReply, request.Payload)
		}

		if request.WantReply {
			request.Reply(true, nil)
		}
	}
}
