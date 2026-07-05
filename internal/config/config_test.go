package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg != (Config{}) {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(`{
		"xray_path": "/xray",
		"sing_box_path": "/sing-box",
		"probe_url": "https://example.com",
		"timeout": "20s"
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.XrayPath != "/xray" || cfg.SingBoxPath != "/sing-box" || cfg.ProbeURL != "https://example.com" || cfg.Timeout != "20s" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestParseTimeout(t *testing.T) {
	timeout, err := ParseTimeout("20s")
	if err != nil {
		t.Fatalf("ParseTimeout returned error: %v", err)
	}
	if timeout != 20*time.Second {
		t.Fatalf("timeout = %s", timeout)
	}
}

func TestParseTimeoutRejectsInvalid(t *testing.T) {
	if _, err := ParseTimeout("soon"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseTimeout("-1s"); err == nil {
		t.Fatal("expected error")
	}
}
