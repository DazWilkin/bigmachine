package gcesystem

import (
	"os/user"
)

// Username returns the current (Unix?) username
// This value is used by System.go to identify the ssh user
// + google_compute_engine
func Username() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// HomeDir returns the current (Unix?) user's home directory
// This value is used System.go to identify the path to the public key to use with ssh
func HomeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}
