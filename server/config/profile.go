package config

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"go.uber.org/zap"
	"hash"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Profile struct {
	name               string
	Username           *regexp.Regexp
	Password           *regexp.Regexp
	passwordHashAlgo   hash.Hash
	NetworkMode        int
	Configurable       bool
	RunLevel           int
	StartupInformation bool
	ExitAfter          string
	KeepOnExit         bool
	Image              string
	ContainerID        string
}

func (p *Profile) Name() string {
	return p.name
}

func (p *Profile) Match(user string, password []byte) bool {
	// username should only be nil if profile was generated from Config.Profile.Dynamic
	if p.Username == nil || p.Username.MatchString(user) {
		if p.passwordHashAlgo != nil {
			password = p.passwordHashAlgo.Sum(password)
		}
		return p.Password.Match(password)
	}
	return false
}

type preProfile struct {
	Username           string
	Password           string
	NetworkMode        int
	Configurable       bool
	RunLevel           int
	StartupInformation bool
	ExitAfter          string
	KeepOnExit         bool
	Image              string
	Container          string
}

func LoadProfileFile(path string, defaultPreProfile preProfile) (Profiles, error) {
	var rawProfile map[string]interface{}
	if _, err := toml.DecodeFile(path, &rawProfile); err != nil {
		return nil, err
	}

	profiles, err := parseRawProfile(rawProfile, path, defaultPreProfile)
	if err != nil {
		return nil, err
	}

	return profiles, nil
}

func LoadProfileDir(path string, defaultPreProfile preProfile) (Profiles, error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var profiles Profiles
	for i, profileConf := range dir {
		p, err := LoadProfileFile(filepath.Join(path, profileConf.Name()), defaultPreProfile)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p...)
		zap.S().Debugf("Pre-loaded file %d (%s) with %d profile(s)", i+1, profileConf.Name(), len(p))
	}

	return profiles, nil
}

func parseRawProfile(rawProfile map[string]interface{}, path string, defaultPreProfile preProfile) (profiles []*Profile, err error) {
	var count int
	for key, value := range rawProfile {
		rawValue, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		pp := preProfile{
			NetworkMode:        3,
			RunLevel:           1,
			StartupInformation: true,
		}
		if err = json.Unmarshal(rawValue, &pp); err != nil {
			return nil, fmt.Errorf("failed to parse %s profile conf file %s: %v", key, path, err)
		}

		var rawUsername string
		if rawUsername = strings.TrimPrefix(pp.Username, "regex:"); rawUsername == pp.Username {
			rawUsername = strings.ReplaceAll(rawUsername, "*", ".*")
		}
		if !strings.HasSuffix(rawUsername, "$") {
			rawUsername += "$"
		}
		username, err := regexp.Compile("(?m)" + rawUsername)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s profile username regex for conf file %s: %v", key, path, err)
		}

		var rawPassword string
		if rawPassword = strings.TrimPrefix(pp.Password, "regex:"); rawUsername == pp.Password {
			rawPassword = strings.ReplaceAll(rawPassword, "*", ".*")
		}
		algo, rawPasswordOrHash := getHash(rawPassword)
		if algo == nil && rawPasswordOrHash == "" {
			rawPasswordOrHash = ".*"
		}
		if !strings.HasSuffix(rawPasswordOrHash, "$") {
			rawPasswordOrHash += "$"
		}
		password, err := regexp.Compile("(?m)" + rawPasswordOrHash)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s profile password regex for conf file %s: %v", key, path, err)
		}

		if (pp.Image == "") == (pp.Container == "") {
			return nil, fmt.Errorf("failed to interpret %s profile image / container definition for conf file %s: `Image` or `Container` must be specified, not both nor none of them", key, path)
		}

		profiles = append(profiles, &Profile{
			name:               key,
			Username:           username,
			Password:           password,
			passwordHashAlgo:   algo,
			NetworkMode:        pp.NetworkMode,
			Configurable:       pp.Configurable,
			RunLevel:           pp.RunLevel,
			StartupInformation: pp.StartupInformation,
			ExitAfter:          pp.ExitAfter,
			KeepOnExit:         pp.KeepOnExit,
			Image:              pp.Image,
			ContainerID:        pp.Container,
		})
		count++
		zap.S().Debugf("Pre-loaded profile %s (%d)", key, count)
	}
	return
}

type Profiles []*Profile

func (ps Profiles) GetByName(name string) (*Profile, bool) {
	for _, profile := range ps {
		if profile.name == name {
			return profile, true
		}
	}
	return nil, false
}

func (ps Profiles) Match(user string, password []byte) (*Profile, bool) {
	for _, profile := range ps {
		if profile.Match(user, password) {
			return profile, true
		}
	}
	return nil, false
}

func DefaultPreProfileFromConfig(config *Config) preProfile {
	defaultProfile := config.Profile.Default

	return preProfile{
		Password:           defaultProfile.Password,
		NetworkMode:        defaultProfile.NetworkMode,
		Configurable:       defaultProfile.Configurable,
		RunLevel:           defaultProfile.RunLevel,
		StartupInformation: defaultProfile.StartupInformation,
		ExitAfter:          defaultProfile.ExitAfter,
		KeepOnExit:         defaultProfile.KeepOnExit,
	}
}

func HardcodedPreProfile() preProfile {
	return preProfile{
		NetworkMode:        3,
		RunLevel:           1,
		StartupInformation: true,
	}
}

func DynamicProfileFromConfig(config *Config, defaultPreProfile preProfile) (Profile, error) {
	raw, err := json.Marshal(config.Profile.Dynamic)
	if err != nil {
		return Profile{}, err
	}
	json.Unmarshal(raw, &defaultPreProfile)

	algo, rawPasswordOrHash := getHash(defaultPreProfile.Password)
	if algo == nil && rawPasswordOrHash == "" {
		rawPasswordOrHash = ".*"
	}
	password, err := regexp.Compile("(?m)" + rawPasswordOrHash)
	if err != nil {
		return Profile{}, fmt.Errorf("failed to parse password regex: %v ", err)
	}

	return Profile{
		name:               "",
		Username:           nil,
		Password:           password,
		passwordHashAlgo:   algo,
		NetworkMode:        defaultPreProfile.NetworkMode,
		Configurable:       defaultPreProfile.Configurable,
		RunLevel:           defaultPreProfile.RunLevel,
		StartupInformation: defaultPreProfile.StartupInformation,
		ExitAfter:          defaultPreProfile.ExitAfter,
		KeepOnExit:         defaultPreProfile.KeepOnExit,
	}, nil
}

func getHash(password string) (algo hash.Hash, raw string) {
	split := strings.SplitN(password, ":", 1)

	if len(split) == 1 {
		return nil, password
	} else {
		raw = split[1]
	}

	switch split[0] {
	case "sha1":
		algo = sha1.New()
	case "sha256":
		algo = sha256.New()
	case "sha512":
		algo = sha512.New()
	default:
		algo = nil
	}
	return
}
