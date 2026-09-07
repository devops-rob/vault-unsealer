package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewVaultClientSystemRoots(t *testing.T) {
	client, err := newVaultClient(TLSConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != requestTimeout {
		t.Fatalf("client.Timeout = %v, want %v", client.Timeout, requestTimeout)
	}
}

func TestNewVaultClientInvalidCACert(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(bad, []byte("this is not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newVaultClient(TLSConfig{CACert: bad}); err == nil {
		t.Fatal("expected error for CA file with no valid certificates, got nil")
	}
}

func TestNewVaultClientMissingCACert(t *testing.T) {
	if _, err := newVaultClient(TLSConfig{CACert: "/no/such/ca.pem"}); err == nil {
		t.Fatal("expected error for missing CA file, got nil")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "ok", cfg: Config{Nodes: []string{"https://127.0.0.1:8200"}, ProbeInterval: 5}},
		{name: "no nodes", cfg: Config{ProbeInterval: 5}, wantErr: true},
		{name: "zero interval", cfg: Config{Nodes: []string{"https://127.0.0.1:8200"}}, wantErr: true},
		{name: "negative interval", cfg: Config{Nodes: []string{"https://x"}, ProbeInterval: -1}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
