package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/src/config"
)

func TestProjectSpecificCachePathDistinct(t *testing.T) {
	p1 := config.GetProjectSpecificCachePath(filepath.Join("/tmp", "x1", "proj"))
	p2 := config.GetProjectSpecificCachePath(filepath.Join("/tmp", "x2", "proj"))
	if p1 == p2 {
		t.Fatalf("expected different paths for same basename different dirs")
	}
}

func TestCacheSetters_Validation(t *testing.T) {
	cfg := config.GetDefaultConfig()
	if err := cfg.SetCacheMemoryLimit(0); err == nil {
		t.Fatalf("expected error for memory limit")
	}
	if err := cfg.SetCacheTTLHours(0); err == nil {
		t.Fatalf("expected error for ttl")
	}
	if err := cfg.SetCacheLanguages([]string{"nosuch"}); err == nil {
		t.Fatalf("expected error for unknown language")
	}
}

func TestLoadConfig_ExpandTildeAndValidate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	storage := filepath.Join("~", ".lsp-gateway", "scip-cache")
	y := []byte("servers:\n  go:\n    command: gopls\n    args: [serve]\ncache:\n  enabled: true\n  storage_path: \"" + storage + "\"\n  max_memory_mb: 64\n  ttl_hours: 24\n  languages: [\"*\"]\n  health_check_minutes: 5\n  eviction_policy: lru\n")
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(cfgPath, y, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Cache == nil || loaded.Cache.StoragePath == "" {
		t.Fatalf("storage path not expanded")
	}
}
