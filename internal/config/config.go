package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pricklywiggles/hone/internal/platform"
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

func FailedURLsPath() string {
	return filepath.Join(DataDir(), "failed_urls.txt")
}

func AppendFailedURL(url string) {
	appendToFile(FailedURLsPath(), url)
}

func appendToFile(path, url string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(url + "\n")
}

func BrowserProfileDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "hone", "browser-profile")
}

// ThresholdsFor returns the fast/normal duration thresholds for the given
// difficulty ("easy", "medium", "hard") from Viper config.
func ThresholdsFor(difficulty string) srs.Thresholds {
	return srs.Thresholds{
		Fast:   viper.GetInt("thresholds." + difficulty + ".fast"),
		Normal: viper.GetInt("thresholds." + difficulty + ".normal"),
	}
}

// BuildURL constructs a problem URL from platform + slug using the configured template.
func BuildURL(platform, slug string) string {
	tmpl := viper.GetString("platforms." + platform + ".url_template")
	if tmpl == "" {
		return "https://" + platform + ".com/problems/" + slug + "/"
	}
	return strings.ReplaceAll(tmpl, "{{slug}}", slug)
}

func setDefaults() {
	viper.SetDefault("thresholds.easy.fast", 10)
	viper.SetDefault("thresholds.easy.normal", 20)
	viper.SetDefault("thresholds.medium.fast", 18)
	viper.SetDefault("thresholds.medium.normal", 30)
	viper.SetDefault("thresholds.hard.fast", 30)
	viper.SetDefault("thresholds.hard.normal", 40)

	viper.SetDefault("auto_focus", true)

	for key, val := range platform.Defaults() {
		viper.SetDefault(key, val)
	}
}
