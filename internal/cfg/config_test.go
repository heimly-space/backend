package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestLoadCreatesConfigWithDefaultsAndRuntimeEnv(t *testing.T) {
	chdirToTempDir(t)
	setRequiredRuntimeEnv(t)
	t.Setenv("PORT", "")
	t.Setenv("SHARE_DATA", "")
	t.Setenv("JWT_SECRET", "")

	cfg := Load()

	if cfg.ID == uuid.Nil {
		t.Fatal("expected non-empty instance id")
	}
	if !cfg.ShareData {
		t.Fatal("expected share_data default true")
	}
	if cfg.JWTSecret == "" {
		t.Fatal("expected generated jwt secret")
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://test-db" {
		t.Fatalf("unexpected database url: %q", cfg.DatabaseURL)
	}
	if cfg.CacheURL != "redis://test-cache/0" {
		t.Fatalf("unexpected cache url: %q", cfg.CacheURL)
	}
	if cfg.StorageURL != "http://test-storage" {
		t.Fatalf("unexpected storage url: %q", cfg.StorageURL)
	}

	if _, err := os.Stat(filepath.Join(configPath, configName)); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestLoadPersistsConfigValuesAcrossRuns(t *testing.T) {
	chdirToTempDir(t)
	setRequiredRuntimeEnv(t)
	t.Setenv("SHARE_DATA", "false")
	t.Setenv("JWT_SECRET", "initial-secret")

	first := Load()

	t.Setenv("SHARE_DATA", "true")
	t.Setenv("JWT_SECRET", "changed-secret")
	second := Load()

	if second.ID != first.ID {
		t.Fatalf("expected same id, got %s then %s", first.ID, second.ID)
	}
	if second.ShareData != first.ShareData {
		t.Fatalf("expected share_data to be persisted, got %t then %t", first.ShareData, second.ShareData)
	}
	if second.JWTSecret != first.JWTSecret {
		t.Fatalf("expected jwt secret to be persisted, got %q then %q", first.JWTSecret, second.JWTSecret)
	}
}

func TestLoadFillsMissingJWTSecretFromEnv(t *testing.T) {
	chdirToTempDir(t)
	setRequiredRuntimeEnv(t)
	t.Setenv("JWT_SECRET", "env-secret")

	if err := os.MkdirAll(configPath, 0o700); err != nil {
		t.Fatalf("mkdir config path: %v", err)
	}

	content := fmt.Sprintf("id: %s\nshare_data: false\n", uuid.New())
	if err := os.WriteFile(filepath.Join(configPath, configName), []byte(content), 0o600); err != nil {
		t.Fatalf("write partial config: %v", err)
	}

	cfg := Load()
	if cfg.JWTSecret != "env-secret" {
		t.Fatalf("expected jwt secret from env, got %q", cfg.JWTSecret)
	}
	if cfg.ShareData {
		t.Fatal("expected share_data from file to stay false")
	}
}

func chdirToTempDir(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func setRequiredRuntimeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://test-db")
	t.Setenv("CACHE_URL", "redis://test-cache/0")
	t.Setenv("STORAGE_URL", "http://test-storage")
}
