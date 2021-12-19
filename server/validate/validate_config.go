package validate

import (
	"docker4ssh/config"
	"docker4ssh/docker"
	"docker4ssh/utils"
	"fmt"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	s "golang.org/x/crypto/ssh"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func NewConfigValidator(cli *client.Client, strict bool, config *config.Config) *ConfigValidator {
	return &ConfigValidator{
		Validator: &Validator{
			Cli:    cli,
			Strict: strict,
		},
		Config: config,
	}
}

type ConfigValidator struct {
	*Validator

	Config *config.Config
}

func (cv *ConfigValidator) Validate() *ValidatorResult {
	errors := make([]*ValidateError, 0)

	errors = append(errors, cv.ValidateProfile().Errors...)
	errors = append(errors, cv.ValidateAPI().Errors...)
	errors = append(errors, cv.ValidateSSH().Errors...)
	errors = append(errors, cv.ValidateDatabase().Errors...)
	errors = append(errors, cv.ValidateNetwork().Errors...)
	errors = append(errors, cv.ValidateLogging().Errors...)

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) ValidateProfile() *ValidatorResult {
	errors := make([]*ValidateError, 0)

	errors = append(errors, cv.validateProfileDefault()...)
	errors = append(errors, cv.validateProfileDynamic()...)

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) validateProfileDefault() []*ValidateError {
	profileDefault := cv.Config.Profile.Default
	errors := make([]*ValidateError, 0)

	if _, err := utils.PasswordToRegex(profileDefault.Password); err != nil {
		errors = append(errors, newValidateError("profile.default", "Password", profileDefault.Password, "not a valid regex string", err))
	}
	networkMode := docker.NetworkMode(profileDefault.NetworkMode)
	if docker.Off > networkMode || networkMode > docker.None {
		errors = append(errors, newValidateError("profile.default", "NetworkMode", profileDefault.NetworkMode, "not a valid network mode", nil))
	}
	runLevel := docker.RunLevel(profileDefault.RunLevel)
	if docker.User > runLevel || runLevel > docker.Forever {
		errors = append(errors, newValidateError("profile.default", "RunLevel", profileDefault.RunLevel, "is not a valid run level", nil))
	}

	return errors
}

func (cv *ConfigValidator) validateProfileDynamic() []*ValidateError {
	profileDynamic := cv.Config.Profile.Dynamic
	errors := make([]*ValidateError, 0)

	if !profileDynamic.Enable && !cv.Strict {
		return errors
	}

	if _, err := utils.PasswordToRegex(profileDynamic.Password); err != nil {
		errors = append(errors, newValidateError("profile.dynamic", "Password", profileDynamic.Password, "not a valid regex string", err))
	}
	networkMode := docker.NetworkMode(profileDynamic.NetworkMode)
	if docker.Off > networkMode || networkMode > docker.None {
		errors = append(errors, newValidateError("profile.dynamic", "NetworkMode", profileDynamic.NetworkMode, "not a valid network mode", nil))
	}
	runLevel := docker.RunLevel(profileDynamic.RunLevel)
	if docker.User > runLevel || runLevel > docker.Forever {
		errors = append(errors, newValidateError("profile.dynamic", "RunLevel", profileDynamic.RunLevel, "is not a valid run level", nil))
	}

	return errors
}

func (cv *ConfigValidator) ValidateAPI() *ValidatorResult {
	api := cv.Config.Api
	errors := make([]*ValidateError, 0)

	if cv.Strict && !isPortFree(api.Port) {
		errors = append(errors, newValidateError("api", "Port", api.Port, "port is already in use", nil))
	}

	errors = append(errors, cv.validateAPIConfigure().Errors...)

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) validateAPIConfigure() *ValidatorResult {
	apiConfigure := cv.Config.Api.Configure
	errors := make([]*ValidateError, 0)

	for k, v := range map[string]string{"Binary": apiConfigure.Binary, "Man": apiConfigure.Man} {
		path := absolutePath("", v)
		if msg, err, ok := fileOk(path); !ok {
			errors = append(errors, newValidateError("api.configure", k, path, msg, err))
		}
	}

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) ValidateSSH() *ValidatorResult {
	ssh := cv.Config.SSH
	errors := make([]*ValidateError, 0)

	if cv.Strict && !isPortFree(ssh.Port) {
		errors = append(errors, newValidateError("api", "Port", ssh.Port, "port is already in use", nil))
	}

	path := absolutePath("", ssh.Keyfile)
	if msg, err, ok := fileOk(path); !ok {
		errors = append(errors, newValidateError("ssh", "Keyfile", path, msg, err))
	} else {
		keyBytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("failed to read file %s: %v", path, err))
		}
		if ssh.Passphrase == "" {
			if _, err = s.ParsePrivateKey(keyBytes); err != nil {
				errors = append(errors, newValidateError("ssh", "Passphrase", ssh.Passphrase, "failed to parse ssh keyfile without password", err))
			}
		} else {
			if _, err = s.ParsePrivateKeyWithPassphrase(keyBytes, []byte(ssh.Passphrase)); err != nil {
				errors = append(errors, newValidateError("ssh", "Passphrase", ssh.Passphrase, "failed to parse ssh keyfile with password", err))
			}
		}
	}

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) ValidateDatabase() *ValidatorResult {
	database := cv.Config.Database
	errors := make([]*ValidateError, 0)

	path := absolutePath("", database.Sqlite3File)
	if msg, err, ok := fileOk(path); !ok {
		errors = append(errors, newValidateError("database", "Sqlite3File", path, msg, err))
	}

	// TODO: implement sql database schema

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) ValidateNetwork() *ValidatorResult {
	network := cv.Config.Network
	errors := make([]*ValidateError, 0)

	if strings.Index(network.Default.Subnet, "/") == -1 {
		errors = append(errors, newValidateError("network.default", "Subnet", network.Default.Subnet, "no network mask is given", nil))
	} else if subnet, _, err := net.ParseCIDR(network.Default.Subnet); err != nil {
		errors = append(errors, newValidateError("network.default", "Subnet", network.Default.Subnet, "invalid subnet ip", err))
	} else if subnet == nil {
		errors = append(errors, newValidateError("network.default", "Subnet", network.Default.Subnet, "invalid subnet ip", nil))
	}

	if strings.Index(network.Isolate.Subnet, "/") == -1 {
		errors = append(errors, newValidateError("network.isolate", "Subnet", network.Isolate.Subnet, "no network mask is given", nil))
	} else if subnet, _, err := net.ParseCIDR(network.Isolate.Subnet); err != nil {
		errors = append(errors, newValidateError("network.isolate", "Subnet", network.Isolate.Subnet, "invalid subnet ip", err))
	} else if subnet == nil {
		errors = append(errors, newValidateError("network.isolate", "Subnet", network.Isolate.Subnet, "invalid subnet ip", nil))
	}

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func (cv *ConfigValidator) ValidateLogging() *ValidatorResult {
	logging := cv.Config.Logging
	errors := make([]*ValidateError, 0)

	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(logging.Level)); err != nil {
		errors = append(errors, newValidateError("logging", "Level", logging.Level, "invalid level", err))
	}
	if cv.Strict {
		path := absolutePath("", logging.OutputFile)
		if msg, err, ok := fileOk(path); !ok {
			errors = append(errors, newValidateError("logging", "OutputFile", logging.OutputFile, msg, err))
		}
		path = absolutePath("", logging.ErrorFile)
		if msg, err, ok := fileOk(path); !ok {
			errors = append(errors, newValidateError("logging", "ErrorFile", logging.ErrorFile, msg, err))
		}
	}

	return &ValidatorResult{
		Strict: cv.Strict,
		Errors: errors,
	}
}

func isPortFree(port uint16) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if listener != nil {
		listener.Close()
	}
	return err == nil && port != 0
}

func absolutePath(parentPath, filePath string) (path string) {
	if filepath.IsAbs(filePath) {
		path = filePath
	} else {
		path = filepath.Join(parentPath, filePath)
	}
	return
}

func fileOk(path string) (string, error, bool) {
	if info, err := os.Stat(path); os.IsNotExist(err) {
		return "file does not exist", err, false
	} else if info.IsDir() {
		return "file is an directory", nil, false
	} else if err != nil {
		return "unexpected error", err, false
	}
	return "", nil, true
}
