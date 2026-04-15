//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

func configDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "hone")
}

func dataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "hone")
}
