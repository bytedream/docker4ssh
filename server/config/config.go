package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

var globConfig *Config

type Config struct {
	Profile struct {
		Dir     string `toml:"Dir"`
		Default struct {
			Password           string `toml:"Password"`
			NetworkMode        int    `toml:"NetworkMode"`
			Configurable       bool   `toml:"Configurable"`
			RunLevel           int    `toml:"RunLevel"`
			StartupInformation bool   `toml:"StartupInformation"`
			ExitAfter          string `toml:"ExitAfter"`
			KeepOnExit         bool   `toml:"KeepOnExit"`
		} `toml:"default"`
		Dynamic struct {
			Enable             bool   `toml:"Enable"`
			Password           string `toml:"Password"`
			NetworkMode        int    `toml:"NetworkMode"`
			Configurable       bool   `toml:"Configurable"`
			RunLevel           int    `toml:"RunLevel"`
			StartupInformation bool   `toml:"StartupInformation"`
			ExitAfter          string `toml:"ExitAfter"`
			KeepOnExit         bool   `toml:"KeepOnExit"`
		} `toml:"dynamic"`
	} `toml:"profile"`
	Api struct {
		Port      uint16 `toml:"Port"`
		Configure struct {
			Binary string `toml:"Binary"`
			Man    string `toml:"Man"`
		} `toml:"configure"`
	} `toml:"api"`
	SSH struct {
		Port       uint16 `toml:"Port"`
		Keyfile    string `toml:"Keyfile"`
		Passphrase string `toml:"Passphrase"`
	} `toml:"ssh"`
	Database struct {
		Sqlite3File string `toml:"Sqlite3File"`
	} `toml:"Database"`
	Network struct {
		Default struct {
			Subnet string `toml:"Subnet"`
		} `toml:"default"`
		Isolate struct {
			Subnet string `toml:"Subnet"`
		} `toml:"isolate"`
	} `toml:"network"`
	Logging struct {
		Level         string `toml:"Level"`
		OutputFile    string `toml:"OutputFile"`
		ErrorFile     string `toml:"ErrorFile"`
		ConsoleOutput bool   `toml:"ConsoleOutput"`
		ConsoleError  bool   `toml:"ConsoleError"`
	} `toml:"logging"`
}

func InitConfig(includeEnv bool) (*Config, error) {
	configFiles := []string{
		"./docker4ssh.conf",
		"~/.docker4ssh",
		"~/.config/docker4ssh.conf",
		"/etc/docker4ssh/docker4ssh.conf",
	}

	for _, file := range configFiles {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			return LoadConfig(file, includeEnv)
		}
	}

	return nil, fmt.Errorf("no speicfied config file (%s) could be found", strings.Join(configFiles, ", "))
}

func LoadConfig(file string, includeEnv bool) (*Config, error) {
	config := &Config{}

	if _, err := toml.DecodeFile(file, config); err != nil {
		return nil, err
	}

	// make paths absolute
	dir := filepath.Dir(file)
	config.Profile.Dir = absoluteFile(dir, config.Profile.Dir)
	config.Api.Configure.Binary = absoluteFile(dir, config.Api.Configure.Binary)
	config.Api.Configure.Man = absoluteFile(dir, config.Api.Configure.Man)
	config.SSH.Keyfile = absoluteFile(dir, config.SSH.Keyfile)
	config.Database.Sqlite3File = absoluteFile(dir, config.Database.Sqlite3File)
	config.Logging.OutputFile = absoluteFile(dir, config.Logging.OutputFile)
	config.Logging.ErrorFile = absoluteFile(dir, config.Logging.ErrorFile)

	if includeEnv {
		if err := updateFromEnv(config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func absoluteFile(path, file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(path, file)
}

// updateFromEnv looks up if specific environment variable are given which can
// also be used to configure the program.
// Every key in the config file can also be specified via environment variables.
// The env variable syntax is SECTION_KEY -> e.g. DEFAULT_PASSWORD or API_PORT
func updateFromEnv(config *Config) error {
	re := reflect.ValueOf(config).Elem()
	rt := re.Type()

	for i := 0; i < re.NumField(); i++ {
		rf := re.Field(i)
		ree := rt.Field(i)

		if err := envParseField(strings.ToUpper(ree.Tag.Get("toml")), rf); err != nil {
			return err
		}
	}
	return nil
}

func envParseField(prefix string, value reflect.Value) error {
	for j := 0; j < value.NumField(); j++ {
		rtt := value.Type().Field(j)
		rff := value.Field(j)

		if rff.Kind() == reflect.Struct {
			if err := envParseField(fmt.Sprintf("%s_%s", prefix, strings.ToUpper(rtt.Tag.Get("toml"))), rff); err != nil {
				return err
			}
			continue
		}

		envName := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(rtt.Tag.Get("toml")))
		val, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}
		var expected string
		switch rff.Kind() {
		case reflect.String:
			rff.SetString(val)
			continue
		case reflect.Bool:
			b, err := strconv.ParseBool(val)
			if err == nil {
				rff.SetBool(b)
				continue
			}
			expected = "true / false (boolean)"
		case reflect.Uint16:
			ui, err := strconv.ParseUint(val, 10, 16)
			if err == nil {
				rff.SetUint(ui)
				continue
			}
			expected = "number (uint16)"
		default:
			return fmt.Errorf("parsed not implemented config type '%s'", rff.Kind())
		}
		return fmt.Errorf("failed to parse environment variable '%s': cannot parse value '%s' as %s", envName, val, expected)
	}
	return nil
}

func GetConfig() *Config {
	return globConfig
}

func SetConfig(config *Config) {
	globConfig = config
}
