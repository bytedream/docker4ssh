package utils

import (
	"regexp"
	"strings"
)

func UsernameToRegex(username string) (*regexp.Regexp, error) {
	var rawUsername string
	if rawUsername = strings.TrimPrefix(username, "regex:"); rawUsername == username {
		rawUsername = strings.ReplaceAll(rawUsername, "*", ".*")
	}
	return regexp.Compile(rawUsername)
}

func PasswordToRegex(password string) (*regexp.Regexp, error) {
	splitPassword := strings.SplitN(password, ":", 1)
	if len(splitPassword) > 1 {
		switch splitPassword[0] {
		case "regex":
			return regexp.Compile(splitPassword[1])
		case "sha1", "sha256", "sha512":
			password = splitPassword[1]
		}
	}
	return regexp.Compile(strings.ReplaceAll(password, "*", ".*"))
}
