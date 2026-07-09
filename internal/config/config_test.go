package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadFileMissingReturnsDefaults(t *testing.T) {
	cfg, err := LoadFile(filepath.Join(t.TempDir(), "missing", "config.json"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if cfg.Language != DefaultLanguage {
		t.Fatalf("Language = %q, want %q", cfg.Language, DefaultLanguage)
	}
	if cfg.Model != DefaultModel {
		t.Fatalf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
}

func TestLoadFilePartialConfigFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"language":"pt-BR"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if cfg.Language != "pt-BR" {
		t.Fatalf("Language = %q, want pt-BR", cfg.Language)
	}
	if cfg.Model != DefaultModel {
		t.Fatalf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Provider != DefaultProvider {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
}

func TestLoadFileInvalidJSONReturnsUsefulError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile() error = nil, want error")
	}
	if got := err.Error(); !strings.HasPrefix(got, "parse config") {
		t.Fatalf("LoadFile() error = %q, want parse config prefix", got)
	}
}

func TestSaveFileCreatesDirectoryAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "commitgen", "config.json")
	want := Config{
		Language: "pt-BR",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKeys: map[string]string{
			"openai": "sk-test",
		},
	}

	if err := SaveFile(path, want); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("LoadFile() = %+v, want %+v", cfg, want)
	}
}
