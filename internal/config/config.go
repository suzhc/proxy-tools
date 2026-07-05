package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const FileName = "px.json"

type Config struct {
	XrayPath    string `json:"xray_path"`
	SingBoxPath string `json:"sing_box_path"`
	ProbeURL    string `json:"probe_url"`
	Timeout     string `json:"timeout"`
}

func LoadForExecutable() (Config, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return Config{}, "", fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	path := filepath.Join(filepath.Dir(exe), FileName)
	cfg, err := Load(path)
	return cfg, path, err
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func ParseTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", raw, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("invalid timeout %q: must be positive", raw)
	}
	return timeout, nil
}
