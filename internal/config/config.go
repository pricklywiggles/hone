package config

import (
	"os"
	"path/filepath"

	"github.com/pricklywiggles/hone/internal/srs"
	"github.com/spf13/viper"
)

func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "hone")
	dataDir := filepath.Join(homeDir, ".local", "share", "hone")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	return nil
}

func DataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "hone")
}

// ThresholdsFor returns the fast/normal duration thresholds for the given
// difficulty ("easy", "medium", "hard") from Viper config.
func ThresholdsFor(difficulty string) srs.Thresholds {
	return srs.Thresholds{
		Fast:   viper.GetInt("thresholds." + difficulty + ".fast"),
		Normal: viper.GetInt("thresholds." + difficulty + ".normal"),
	}
}

func setDefaults() {
	viper.SetDefault("thresholds.easy.fast", 10)
	viper.SetDefault("thresholds.easy.normal", 20)
	viper.SetDefault("thresholds.medium.fast", 15)
	viper.SetDefault("thresholds.medium.normal", 30)
	viper.SetDefault("thresholds.hard.fast", 20)
	viper.SetDefault("thresholds.hard.normal", 40)

	viper.SetDefault("platforms.leetcode.url_template", "https://leetcode.com/problems/{{slug}}/")
	viper.SetDefault("platforms.neetcode.url_template", "https://neetcode.io/problems/{{slug}}/question")
}
