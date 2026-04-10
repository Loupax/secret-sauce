package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Timeout     string `json:"timeout"`               // e.g. "15m" — idle timeout for daemon
	AutoSpawn   bool   `json:"auto_spawn"`            // if true, CLI auto-starts daemon when socket is absent
	Concurrency int    `json:"concurrency,omitempty"` // max parallel writes; 0 = fall through to runtime.NumCPU()
}

func defaults() *Config {
	return &Config{
		Timeout:   "15m",
		AutoSpawn: true,
	}
}

func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return defaults(), nil
	}

	path := filepath.Join(configDir, "secret-sauce", "config.json")
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults(), nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := defaults()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
