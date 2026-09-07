package main

import (
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestParseKeys(t *testing.T) {
	got := parseKeys("  key1 \n\nkey2\n   \nkey3\n")
	want := []string{"key1", "key2", "key3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseKeys = %v, want %v", got, want)
	}
}

func TestEnvKeyProvider(t *testing.T) {
	t.Setenv("UNSEAL_KEY_1", "aaa")
	t.Setenv("UNSEAL_KEY_2", " bbb ") // surrounding whitespace should be trimmed

	p := &envKeyProvider{vars: []string{"UNSEAL_KEY_1", "UNSEAL_KEY_2"}}
	keys, err := p.UnsealKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"aaa", "bbb"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("UnsealKeys = %v, want %v", keys, want)
	}
}

func TestEnvKeyProviderMissingVar(t *testing.T) {
	p := &envKeyProvider{vars: []string{"DEFINITELY_NOT_SET_12345"}}
	if _, err := p.UnsealKeys(context.Background()); err == nil {
		t.Fatal("expected error for unset environment variable, got nil")
	}
}

func TestExecKeyProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("printf-based test not portable to Windows")
	}
	p := &execKeyProvider{
		command: []string{"printf", "%s\n%s\n%s\n", "k1", "k2", "k3"},
		timeout: 5 * time.Second,
	}
	keys, err := p.UnsealKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"k1", "k2", "k3"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("UnsealKeys = %v, want %v", keys, want)
	}
}

func TestExecKeyProviderCommandFailure(t *testing.T) {
	p := &execKeyProvider{command: []string{"false"}, timeout: 5 * time.Second}
	if _, err := p.UnsealKeys(context.Background()); err == nil {
		t.Fatal("expected error when command exits non-zero, got nil")
	}
}

func TestExecKeyProviderEmptyOutput(t *testing.T) {
	p := &execKeyProvider{command: []string{"true"}, timeout: 5 * time.Second}
	if _, err := p.UnsealKeys(context.Background()); err == nil {
		t.Fatal("expected error when command produces no keys, got nil")
	}
}

func TestNewKeyProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     KeySourceConfig
		wantErr bool
		wantT   any
	}{
		{name: "env ok", cfg: KeySourceConfig{Type: "env", EnvVars: []string{"A"}}, wantT: &envKeyProvider{}},
		{name: "env missing vars", cfg: KeySourceConfig{Type: "env"}, wantErr: true},
		{name: "exec ok", cfg: KeySourceConfig{Type: "exec", Command: []string{"echo", "x"}}, wantT: &execKeyProvider{}},
		{name: "exec missing command", cfg: KeySourceConfig{Type: "exec"}, wantErr: true},
		{name: "empty type", cfg: KeySourceConfig{}, wantErr: true},
		{name: "unknown type", cfg: KeySourceConfig{Type: "file"}, wantErr: true},
		{name: "case insensitive", cfg: KeySourceConfig{Type: "ENV", EnvVars: []string{"A"}}, wantT: &envKeyProvider{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newKeyProvider(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got provider %T", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if reflect.TypeOf(p) != reflect.TypeOf(tc.wantT) {
				t.Fatalf("provider type = %T, want %T", p, tc.wantT)
			}
		})
	}
}

func TestExecKeyProviderDefaultTimeout(t *testing.T) {
	p, err := newKeyProvider(KeySourceConfig{Type: "exec", Command: []string{"echo", "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ep, ok := p.(*execKeyProvider)
	if !ok {
		t.Fatalf("expected *execKeyProvider, got %T", p)
	}
	if ep.timeout != defaultExecTimeout {
		t.Fatalf("timeout = %v, want %v", ep.timeout, defaultExecTimeout)
	}
}
