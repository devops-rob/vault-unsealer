package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hcl")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigEnvSource(t *testing.T) {
	path := writeConfig(t, `
log_level      = "debug"
probe_interval = 7
nodes          = ["https://10.0.0.1:8200", "https://10.0.0.2:8200"]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

unseal_key_source {
  type     = "env"
  env_vars = ["K1", "K2", "K3"]
}
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" || cfg.ProbeInterval != 7 {
		t.Fatalf("scalars = %q/%d", cfg.LogLevel, cfg.ProbeInterval)
	}
	if !reflect.DeepEqual(cfg.Nodes, []string{"https://10.0.0.1:8200", "https://10.0.0.2:8200"}) {
		t.Fatalf("nodes = %v", cfg.Nodes)
	}
	if cfg.TLS == nil || cfg.TLS.CACert != "/etc/vault-unsealer/ca.pem" || cfg.TLS.SkipVerify {
		t.Fatalf("tls = %+v", cfg.TLS)
	}
	if len(cfg.KeySources) != 1 || cfg.KeySources[0].Type != "env" ||
		!reflect.DeepEqual(cfg.KeySources[0].EnvVars, []string{"K1", "K2", "K3"}) {
		t.Fatalf("key sources = %+v", cfg.KeySources)
	}
}

func TestLoadConfigMultipleSources(t *testing.T) {
	path := writeConfig(t, `
nodes = ["https://10.0.0.1:8200"]

unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1"]
}

unseal_key_source {
  type    = "exec"
  command = ["/opt/fetch-1password.sh"]
}

unseal_key_source {
  type    = "exec"
  command = ["/opt/fetch-nomad.sh"]
}
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.KeySources) != 3 {
		t.Fatalf("expected 3 key sources, got %d", len(cfg.KeySources))
	}
	if cfg.KeySources[0].Type != "env" || cfg.KeySources[1].Type != "exec" || cfg.KeySources[2].Type != "exec" {
		t.Fatalf("unexpected source types: %+v", cfg.KeySources)
	}
	// The parsed sources should build a working aggregate provider.
	if _, err := newKeyProviders(cfg.KeySources); err != nil {
		t.Fatalf("newKeyProviders: %v", err)
	}
}

func TestLoadConfigExecSourceDefaults(t *testing.T) {
	path := writeConfig(t, `
nodes = ["https://10.0.0.1:8200"]

unseal_key_source {
  type    = "exec"
  command = ["/usr/local/bin/fetch-keys.sh"]
}
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Defaults applied.
	if cfg.LogLevel != "info" {
		t.Fatalf("default log_level = %q, want info", cfg.LogLevel)
	}
	if cfg.ProbeInterval != 10 {
		t.Fatalf("default probe_interval = %d, want 10", cfg.ProbeInterval)
	}
	// Optional tls block omitted.
	if cfg.TLS != nil {
		t.Fatalf("tls = %+v, want nil", cfg.TLS)
	}
	if len(cfg.KeySources) != 1 || cfg.KeySources[0].Type != "exec" ||
		!reflect.DeepEqual(cfg.KeySources[0].Command, []string{"/usr/local/bin/fetch-keys.sh"}) {
		t.Fatalf("key sources = %+v", cfg.KeySources)
	}

	// The parsed config should build a working provider.
	if _, err := newKeyProviders(cfg.KeySources); err != nil {
		t.Fatalf("newKeyProviders: %v", err)
	}
}

func TestLoadConfigMissingKeySource(t *testing.T) {
	path := writeConfig(t, `
nodes = ["https://10.0.0.1:8200"]
`)
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected error for missing unseal_key_source block, got nil")
	}
}

func TestLoadConfigInvalidHCL(t *testing.T) {
	path := writeConfig(t, `this is = not valid { hcl`)
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected parse error for invalid HCL, got nil")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := loadConfig("/no/such/config.hcl"); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
