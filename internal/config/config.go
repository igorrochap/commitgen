package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultLanguage = "en"
	DefaultModel    = "gemma4:31b-cloud"
	DefaultProvider = "ollama"
)

type Config struct {
	Language string            `json:"language,omitempty"`
	Model    string            `json:"model,omitempty"`
	Provider string            `json:"provider,omitempty"`
	APIKeys  map[string]string `json:"api_keys,omitempty"`
}

func IsSupportedProvider(provider string) bool {
	switch provider {
	case "ollama", "openai", "anthropic", "gemini":
		return true
	default:
		return false
	}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "commitgen", "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	return LoadFile(path)
}

func LoadFile(path string) (Config, error) {
	cfg := Defaults()
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var saved Config
	if err := json.Unmarshal(content, &saved); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if saved.Language != "" {
		cfg.Language = saved.Language
	}
	if saved.Model != "" {
		cfg.Model = saved.Model
	}
	if saved.Provider != "" {
		cfg.Provider = saved.Provider
	}
	if len(saved.APIKeys) > 0 {
		cfg.APIKeys = saved.APIKeys
	}
	return cfg, nil
}

func SaveFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Defaults() Config {
	return Config{
		Language: DefaultLanguage,
		Model:    DefaultModel,
		Provider: DefaultProvider,
		APIKeys:  map[string]string{},
	}
}
