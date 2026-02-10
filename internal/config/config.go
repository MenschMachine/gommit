package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Provider        string `toml:"provider"`
	Model           string `toml:"model"`
	BaseURL         string `toml:"base_url"`
	Style           string `toml:"style"`
	SplitThreshold  int    `toml:"split_threshold"`
	PerFileLimit    int    `toml:"per_file_limit"`
	OpenRouterRef   string `toml:"openrouter_referer"`
	OpenRouterTitle string `toml:"openrouter_title"`
}

func DefaultConfig() Config {
	return Config{
		Provider:        "openai",
		Model:           "",
		BaseURL:         "",
		Style:           "conventional",
		SplitThreshold:  200000,
		PerFileLimit:    20000,
		OpenRouterRef:   "",
		OpenRouterTitle: "",
	}
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gommit", "config.toml"), nil
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if info.IsDir() {
		return cfg, fmt.Errorf("config path is a directory: %s", path)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func ApplyEnvOverrides(cfg *Config) {
	setStringEnv(&cfg.Provider, "GOMMIT_PROVIDER")
	setStringEnv(&cfg.Model, "GOMMIT_MODEL")
	setStringEnv(&cfg.BaseURL, "GOMMIT_BASE_URL")
	setStringEnv(&cfg.Style, "GOMMIT_STYLE")
	setIntEnv(&cfg.SplitThreshold, "GOMMIT_SPLIT_THRESHOLD")
	setIntEnv(&cfg.PerFileLimit, "GOMMIT_PER_FILE_LIMIT")
	setStringEnv(&cfg.OpenRouterRef, "GOMMIT_OPENROUTER_REFERER")
	setStringEnv(&cfg.OpenRouterTitle, "GOMMIT_OPENROUTER_TITLE")
	setStringEnv(&cfg.OpenRouterRef, "OPENROUTER_REFERER")
	setStringEnv(&cfg.OpenRouterTitle, "OPENROUTER_TITLE")
}

func setStringEnv(target *string, key string) {
	val := strings.TrimSpace(os.Getenv(key))
	if val != "" {
		*target = val
	}
}

func setIntEnv(target *int, key string) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return
	}
	*target = parsed
}

func ResolveAPIKey(provider string) (string, error) {
	keys := []string{"GOMMIT_API_KEY"}
	switch strings.ToLower(provider) {
	case "openai":
		keys = append([]string{"OPENAI_API_KEY"}, keys...)
	case "openrouter":
		keys = append([]string{"OPENROUTER_API_KEY"}, keys...)
	case "anthropic":
		keys = append([]string{"ANTHROPIC_API_KEY"}, keys...)
	}
	for _, key := range keys {
		val := strings.TrimSpace(os.Getenv(key))
		if val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("missing API key for provider %q", provider)
}

func DefaultBaseURL(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "anthropic":
		return ""
	default:
		return ""
	}
}
