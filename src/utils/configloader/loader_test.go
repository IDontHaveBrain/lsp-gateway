package configloader

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
	"lsp-gateway/src/config"
)

func TestLoadOrAuto_DefaultPathRespected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	defPath := filepath.Join(tmp, ".lsp-gateway", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(defPath), 0755); err != nil {
		t.Fatalf("mk: %v", err)
	}
	data := []byte("servers:\n  go:\n    command: gopls\n    args: [serve]\ncache:\n  enabled: true\n  storage_path: \"" + filepath.Join(tmp, "cache") + "\"\n  max_memory_mb: 64\n  ttl_hours: 24\n  languages: [\"go\"]\n  health_check_minutes: 5\n  eviction_policy: lru\n")
	if err := os.WriteFile(defPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg := LoadOrAuto("")
	if cfg == nil || cfg.Servers["go"].Command != "gopls" {
		t.Fatalf("did not load default path config")
	}
}

func TestLoadForServer_AutoDetectInstalledJDTLS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path/exec mode differs on windows")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	tools := filepath.Join(tmp, ".lsp-gateway", "tools", "java", "bin")
	if err := os.MkdirAll(tools, 0755); err != nil {
		t.Fatalf("mk: %v", err)
	}
	jdt := filepath.Join(tools, "jdtls")
	if err := os.WriteFile(jdt, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("PATH", "")
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	cfg := &config.Config{Servers: map[string]*config.ServerConfig{"java": {Command: "jdtls"}}, Cache: config.GetDefaultCacheConfig()}
	b, _ := yaml.Marshal(struct {
		Servers map[string]*config.ServerConfig `yaml:"servers"`
		Cache   *config.CacheConfig             `yaml:"cache"`
	}{Servers: cfg.Servers, Cache: cfg.Cache})
	if err := os.WriteFile(cfgPath, b, 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	loaded := LoadForServer(cfgPath)
	if loaded.Servers["java"].Command == "jdtls" {
		t.Fatalf("expected auto-detected installed jdtls path, got %s", loaded.Servers["java"].Command)
	}
}
