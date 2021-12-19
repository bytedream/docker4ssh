package docker

import (
	"archive/tar"
	"bytes"
	"context"
	c "docker4ssh/config"
	"docker4ssh/database"
	"docker4ssh/terminal"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func simpleContainerFromID(ctx context.Context, client *Client, config Config, containerID string) (*SimpleContainer, error) {
	inspect, err := client.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	sc := &SimpleContainer{
		config: config,
		Image: Image{
			ref: inspect.Image,
		},
		ContainerID:     containerID[:12],
		FullContainerID: containerID,
		client:          client,
		cli:             client.Client,
	}

	sc.init(ctx)

	return sc, nil
}

// newSimpleContainer creates a new container.
// Currently, only for internal usage, may be changing in future
func newSimpleContainer(ctx context.Context, client *Client, config Config, image Image, containerName string) (*SimpleContainer, error) {
	// create a new container from the given image and activate in- and output
	resp, err := client.Client.ContainerCreate(ctx, &container.Config{
		Image:        image.Ref(),
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		AttachStdout: true,
		OpenStdin:    true,
	}, nil, nil, nil, containerName)
	if err != nil {
		return nil, err
	}

	sc := &SimpleContainer{
		config:          config,
		Image:           image,
		ContainerID:     resp.ID[:12],
		FullContainerID: resp.ID,
		client:          client,
		cli:             client.Client,
	}

	sc.init(ctx)

	return sc, nil
}

// SimpleContainer is the basic struct to control a docker4ssh container
type SimpleContainer struct {
	config          Config
	Image           Image
	ContainerID     string
	FullContainerID string

	started bool

	cancel context.CancelFunc

	client *Client

	// cli is just a shortcut for Client.Client
	cli *client.Client

	Network struct {
		ID string
		IP string
	}
}

func (sc *SimpleContainer) init(ctx context.Context) {
	// disconnect from default docker network
	sc.cli.NetworkDisconnect(ctx, sc.client.Network[Host], sc.FullContainerID, true)
}

// Start starts the container
func (sc *SimpleContainer) Start(ctx context.Context) error {
	if err := sc.cli.ContainerStart(ctx, sc.FullContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	if !sc.started {
		// initializes all settings.
		// as third argument is a pseudo empty used to
		// call every function in SimpleContainer.updateConfig.
		// for the same reason Config.Configurable and
		// Config.KeepOnExit are negated from their value in
		// sc.config
		if err := sc.updateConfig(ctx, Config{
			Configurable: !sc.config.Configurable,
			KeepOnExit:   !sc.config.KeepOnExit,
		}, sc.config); err != nil {
			return err
		}
		sc.started = true
	}

	return nil
}

// Stop stops the container
func (sc *SimpleContainer) Stop(ctx context.Context) error {
	timeout := 0 * time.Second
	if err := sc.cli.ContainerStop(ctx, sc.FullContainerID, &timeout); err != nil {
		return err
	}

	if !sc.config.KeepOnExit {
		if err := sc.cli.ContainerRemove(ctx, sc.FullContainerID, types.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
		// delete all references to the container in the database
		return sc.client.Database.Delete(sc.FullContainerID)
	}
	return nil
}

func (sc *SimpleContainer) Running(ctx context.Context) (bool, error) {
	resp, err := sc.cli.ContainerInspect(ctx, sc.FullContainerID)
	if err != nil {
		return false, err
	}
	return resp.State != nil && resp.State.Running, nil
}

// WaitUntilStop waits until the container stops running
func (sc *SimpleContainer) WaitUntilStop(ctx context.Context) error {
	statusChan, errChan := sc.cli.ContainerWait(ctx, sc.FullContainerID, container.WaitConditionNotRunning)
	select {
	case err := <-errChan:
		return err
	case <-statusChan:
	}
	return nil
}

// ExecuteConn executes a command in the container and returns the connection to the output
func (sc *SimpleContainer) ExecuteConn(ctx context.Context, command string, args ...string) (net.Conn, error) {
	execID, err := sc.cli.ContainerExecCreate(ctx, sc.FullContainerID, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string{command}, args...),
	})
	resp, err := sc.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	return resp.Conn, err
}

// Execute executes a command in the container and returns the response after finished
func (sc *SimpleContainer) Execute(ctx context.Context, command string, args ...string) ([]byte, error) {
	buf := bytes.Buffer{}

	conn, err := sc.ExecuteConn(ctx, command, args...)
	if err != nil {
		return nil, err
	}

	io.Copy(&buf, conn)

	return buf.Bytes(), nil
}

// CopyFrom copies a file from the host system to the client.
// Normal files and directories are accepted
func (sc *SimpleContainer) CopyFrom(ctx context.Context, src, dst string) error {
	r, _, err := sc.cli.CopyFromContainer(ctx, sc.FullContainerID, src)
	if err != nil {
		return err
	}
	defer r.Close()

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); os.IsNotExist(err) {
				if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err = io.Copy(f, tr); err != nil {
				return err
			}
			_ = f.Close()
		}
	}
}

// CopyTo copies a file from the container to host.
// Normal files and directories are accepted
func (sc *SimpleContainer) CopyTo(ctx context.Context, src, dst string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		err = filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}
			header.Name = strings.TrimPrefix(strings.TrimPrefix(path, src), "/")

			// write every file to the container.
			// it might be better to write the file content to a buffer or
			// store the file pointer in a slice and write the buffer / stored
			// file pointer to the tar writer when every file was walked
			//
			// TODO: Test if the two described methods are better than sending every file on it's own
			buf := &bytes.Buffer{}

			tw := tar.NewWriter(buf)
			if err = tw.WriteHeader(header); err != nil {
				return err
			}
			defer tw.Close()

			io.Copy(tw, file)

			err = sc.cli.CopyToContainer(ctx, sc.FullContainerID, dst, buf, types.CopyToContainerOptions{
				AllowOverwriteDirWithFile: true,
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		file, err := os.Open(src)
		if err != nil {
			return err
		}
		info, err := os.Lstat(src)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.Base(src)

		buf := &bytes.Buffer{}
		tw := tar.NewWriter(buf)
		if err = tw.WriteHeader(header); err != nil {
			return err
		}
		defer tw.Close()

		_, _ = io.Copy(tw, file)

		err = sc.cli.CopyToContainer(ctx, sc.FullContainerID, dst, buf, types.CopyToContainerOptions{
			AllowOverwriteDirWithFile: true,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Config returns the current container config
func (sc *SimpleContainer) Config() Config {
	return sc.config
}

// UpdateConfig updates the container config
func (sc *SimpleContainer) UpdateConfig(ctx context.Context, config Config) error {
	oldConfig := sc.config

	if err := sc.updateConfig(ctx, oldConfig, config); err != nil {
		return err
	}

	var ocm, ncm, sm map[string]interface{}
	sm = make(map[string]interface{}, 0)

	ocj, _ := json.Marshal(oldConfig)
	ncj, _ := json.Marshal(config)

	json.Unmarshal(ocj, &ocm)
	json.Unmarshal(ncj, &ncm)

	srt := reflect.TypeOf(database.Settings{})

	for k, v := range ocm {
		newValue := ncm[k]
		if v != newValue && newValue != nil {
			field, ok := srt.FieldByName(k)
			if !ok {
				continue
			}

			sm[field.Tag.Get("json")] = newValue
		}
	}

	// marshal the map into new settings
	var settings database.Settings
	body, _ := json.Marshal(sm)
	json.Unmarshal(body, &settings)

	err := sc.client.Database.SetSettings(sc.FullContainerID, settings)
	if err != nil {
		return err
	}

	if config.KeepOnExit {
		if _, ok := sc.client.Database.GetAuthByContainer(sc.FullContainerID); !ok {
			if err = sc.client.Database.SetAuth(sc.FullContainerID, database.Auth{
				User: &sc.ContainerID,
			}); err != nil {
				return err
			}
		}
	}
	sc.config = config

	return nil
}

func (sc *SimpleContainer) updateConfig(ctx context.Context, oldConfig, newConfig Config) error {
	if newConfig.NetworkMode != oldConfig.NetworkMode {
		if err := sc.setNetworkMode(ctx, oldConfig.NetworkMode, newConfig.NetworkMode, sc.client.Network != nil); err != nil {
			return err
		}
		zap.S().Debugf("Set network mode for %s to %s", sc.ContainerID, newConfig.NetworkMode.Name())
	}
	if newConfig.Configurable != oldConfig.Configurable {
		if err := sc.setConfigurable(ctx, newConfig.Configurable); err != nil {
			return err
		}
		zap.S().Debugf("Set configurable for %s to %t", sc.ContainerID, newConfig.Configurable)
	}
	if newConfig.ExitAfter != oldConfig.ExitAfter {
		sc.setExitAfterListener(ctx, newConfig.RunLevel, newConfig.ExitAfter)
		zap.S().Debugf("Set exit after listener for %s", sc.ContainerID)
	}

	sc.config = newConfig
	return nil
}

// setNetworkMode changes the network mode for the container
func (sc *SimpleContainer) setNetworkMode(ctx context.Context, oldMode, newMode NetworkMode, networking bool) error {
	var networkID string

	if !networking {
		networkID = sc.client.Network[Off]
	} else {
		networkID = sc.client.Network[newMode]
	}

	if networkID != "" {
		sc.cli.NetworkDisconnect(ctx, sc.client.Network[oldMode], sc.FullContainerID, true)
		// connect container to a network
		if err := sc.cli.NetworkConnect(ctx, networkID, sc.FullContainerID, &network.EndpointSettings{}); err != nil {
			return err
		}
	}

	// inspect the container to get its ip address (yes i was too lazy to implement
	// a service that generates the ips without docker)
	resp, err := sc.cli.ContainerInspect(ctx, sc.FullContainerID)
	if err != nil {
		return err
	}
	// update the internal network information
	sc.Network.ID = networkID
	sc.Network.IP = resp.NetworkSettings.Networks[newMode.NetworkName()].IPAddress

	return nil
}

func (sc *SimpleContainer) setConfigurable(ctx context.Context, configurable bool) error {
	cconfig := c.GetConfig()

	if configurable {
		for srcFile, dstDir := range map[string]string{cconfig.Api.Configure.Binary: "/bin", cconfig.Api.Configure.Man: "/usr/share/man/man1"} {
			if err := sc.CopyTo(ctx, srcFile, dstDir); err != nil {
				if strings.HasSuffix(dstDir, "/man1") {
					// man files aren't that necessary, so if the copy fails it throws only a warning.
					// this error gets thrown when the container is alpine linux, for example.
					// it does not have a /usr/share/man/man1 directory and the copy fails
					// TODO: Create a directory if not existing to prevent this error
					zap.S().Warnf("Failed to copy %s to %s/%s for %s: %v", srcFile, dstDir, filepath.Base(srcFile), sc.ContainerID, err)
					continue
				} else {
					return fmt.Errorf("failed to copy %s to %s/%s for %s: %v", srcFile, dstDir, filepath.Base(srcFile), sc.ContainerID, err)
				}
			}
			zap.S().Debugf("Copied %s to %s (%s)", srcFile, filepath.Join(dstDir, filepath.Base(srcFile)), sc.ContainerID)
		}
		resp, err := sc.cli.ContainerInspect(ctx, sc.FullContainerID)
		if err != nil {
			return err
		}
		_, err = sc.Execute(ctx, "sh", "-c", fmt.Sprintf("echo -n %s:%d > /etc/docker4ssh", resp.NetworkSettings.Networks[sc.config.NetworkMode.NetworkName()].Gateway, cconfig.Api.Port))
		if err != nil {
			return err
		}
		zap.S().Debugf("Set ip and port of server for %s", sc.ContainerID)
	} else {
		_, err := sc.Execute(ctx, "rm",
			"-rf",
			fmt.Sprintf("/bin/%s", filepath.Base(cconfig.Api.Configure.Binary)),
			fmt.Sprintf("/usr/share/man/man1/%s", filepath.Base(cconfig.Api.Configure.Man)),
			"/etc/docker4ssh")
		if err != nil {
			return err
		}
		zap.S().Debugf("Removed all configurable related files from %s", sc.ContainerID)
	}

	return nil
}

// setAPIRoute sets the IP and port for docker container tools
func (sc *SimpleContainer) setAPIRoute(ctx context.Context, activate bool) error {
	var err error
	if activate {
		var resp types.ContainerJSON
		resp, err = sc.cli.ContainerInspect(ctx, sc.FullContainerID)
		if err != nil {
			return err
		}
		cconfig := c.GetConfig()
		if resp.NetworkSettings != nil {
			_, err = sc.Execute(ctx, "sh", "-c", fmt.Sprintf("echo -n %s:%d > /etc/docker4ssh", resp.NetworkSettings.Networks[sc.config.NetworkMode.NetworkName()].Gateway, cconfig.Api.Port))
		}
	} else {
		_, err = sc.Execute(ctx, "rm", "-rf", "/etc/docker4ssh")
	}
	return err
}

// setExitAfterListener listens for exit after processes
func (sc *SimpleContainer) setExitAfterListener(ctx context.Context, runlevel RunLevel, process string) {
	if sc.cancel != nil {
		sc.cancel()
	}

	if process == "" {
		return
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	sc.cancel = cancel

	go func() {
		var rawPid []byte
		var err error

		// check for the pid of Config.ExitAfter and wait 1 second if it wasn't found
		for {
			rawPid, err = sc.Execute(cancelCtx, "pidof", "-s", process)
			if len(rawPid) > 0 || err != nil {
				break
			}
			time.Sleep(1 * time.Second)
		}

		// sometimes garbage bytes are sent as well, they are getting filtered here
		var pid []byte
		for _, b := range rawPid {
			if b > '0' && b < '9' {
				pid = append(pid, b)
			}
		}

		pid = bytes.TrimSuffix(pid, []byte("\n"))

		if _, err = sc.Execute(cancelCtx, "sh", "-c", fmt.Sprintf("tail --pid=%s -f /dev/null", pid)); err != nil && cancelCtx.Err() == nil {
			zap.S().Errorf("Could not wait on process %s (%s) for %s", process, pid, sc.ContainerID)
			return
		}

		if runlevel != Forever {
			sc.Stop(context.Background())
		}
	}()
}

func InteractiveContainerFromID(ctx context.Context, client *Client, config Config, containerID string) (*InteractiveContainer, error) {
	sc, err := simpleContainerFromID(ctx, client, config, containerID)
	if err != nil {
		return nil, err
	}
	return &InteractiveContainer{
		SimpleContainer: sc,
	}, nil
}

func NewInteractiveContainer(ctx context.Context, cli *Client, config Config, image Image, containerName string) (*InteractiveContainer, error) {
	sc, err := newSimpleContainer(ctx, cli, config, image, containerName)
	if err != nil {
		return nil, err
	}
	return &InteractiveContainer{
		SimpleContainer: sc,
	}, nil
}

type InteractiveContainer struct {
	*SimpleContainer

	terminalCount int
}

// TerminalCount returns the count of active terminals
func (ic *InteractiveContainer) TerminalCount() int {
	return ic.terminalCount
}

// Terminal creates a new interactive terminal session for the container
func (ic *InteractiveContainer) Terminal(ctx context.Context, term *terminal.Terminal) error {
	// get the default shell for the root user
	rawShell, err := ic.Execute(ctx, "sh", "-c", "getent passwd root | cut -d : -f 7")
	if err != nil {
		return err
	}

	// here we cut out only newlines (which also could've been done via
	// bytes.ReplaceAll or strings.ReplaceAll) and redundant bytes
	// which sometimes get returned too and which cannot be interpreted
	// by the docker engine
	shell := bytes.Buffer{}
	for _, b := range rawShell {
		if b > ' ' {
			shell.WriteByte(b)
		}
	}

	id, err := ic.cli.ContainerExecCreate(ctx, ic.FullContainerID, types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{shell.String()},
	})
	if err != nil {
		return err
	}

	resp, err := ic.cli.ContainerExecAttach(ctx, id.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		return err
	}
	errChan := make(chan error)

	go func() {
		// copy every input to the container
		if _, err = io.Copy(term, resp.Conn); err != nil {
			errChan <- err
		}
		errChan <- nil
	}()
	go func() {
		// copy every output from the container
		if _, err = io.Copy(resp.Conn, term); err != nil {
			errChan <- err
		}
		errChan <- nil
	}()

	ic.terminalCount++
	select {
	case err = <-errChan:
		resp.Conn.Close()
	}
	ic.terminalCount--

	return err
}
