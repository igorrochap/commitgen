// Package config loads and saves commitgen configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultLanguage is the language used when no language is configured.
	DefaultLanguage = "en"
	// DefaultModel is the model used when no model is configured.
	DefaultModel = "gemma4:31b-cloud"
	// DefaultProvider is the provider used when no provider is configured.
	DefaultProvider = "ollama"
)

// Config contains the preferences and credentials used by commitgen.
type Config struct {
	// Language is the language used to generate commit messages.
	Language string `json:"language,omitempty"`
	// Model is the model used by the selected provider.
	Model string `json:"model,omitempty"`
	// Provider is the service used to generate commit messages.
	Provider string `json:"provider,omitempty"`
	// APIKeys contains credentials keyed by provider name.
	APIKeys map[string]string `json:"api_keys,omitempty"`
}

// IsSupportedProvider reports whether provider is supported by commitgen.
func IsSupportedProvider(provider string) bool {
	switch provider {
	case "ollama", "openai", "anthropic", "gemini":
		return true
	default:
		return false
	}
}

// Path returns the path to the user's commitgen configuration file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "commitgen", "config.json"), nil
}

// Load reads the user's configuration, returning defaults when no file exists.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	return LoadFile(path)
}

// LoadFile reads configuration from path, returning defaults when it does not exist.
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

// SaveFile writes cfg to path, creating its parent directory when needed.
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

// Defaults returns the default configuration.
func Defaults() Config {
	return Config{
		Language: DefaultLanguage,
		Model:    DefaultModel,
		Provider: DefaultProvider,
		APIKeys:  map[string]string{},
	}
}
