//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".config", "hone")
	}
	return filepath.Join(dir, "hone")
}

func dataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".local", "share", "hone")
	}
	return filepath.Join(dir, "hone")
}
