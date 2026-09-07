package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// Config is the non-secret runtime configuration for vault-unsealer, parsed from
// an HCL file. Unseal keys are intentionally NOT part of this struct: they are
// resolved at runtime from an external source (see KeySourceConfig) so that
// secrets never live in the configuration file.
type Config struct {
	LogLevel      string            `hcl:"log_level,optional"`
	Nodes         []string          `hcl:"nodes"`
	ProbeInterval int               `hcl:"probe_interval,optional"`
	// DisableMlock skips locking the process's memory into RAM. Locking is on by
	// default to keep unseal keys out of swap; set this to true only when the
	// deployment cannot grant CAP_IPC_LOCK and relies on disabled/encrypted swap
	// instead.
	DisableMlock bool              `hcl:"disable_mlock,optional"`
	TLS          *TLSConfig        `hcl:"tls,block"`
	KeySources   []KeySourceConfig `hcl:"unseal_key_source,block"`
}

// TLSConfig controls how vault-unsealer connects to Vault's HTTPS API.
type TLSConfig struct {
	// CACert is the path to a PEM bundle used to verify the Vault server
	// certificate. When empty, the system trust store is used.
	CACert string `hcl:"ca_cert,optional"`
	// SkipVerify disables TLS certificate verification. This is insecure and
	// should only ever be used for local testing.
	SkipVerify bool `hcl:"skip_verify,optional"`
}

// KeySourceConfig selects where unseal keys are fetched from at runtime.
type KeySourceConfig struct {
	// Type is the provider type: "env" or "exec".
	Type string `hcl:"type"`
	// EnvVars is the list of environment variable names to read (type=env),
	// one unseal key per variable.
	EnvVars []string `hcl:"env_vars,optional"`
	// Command is the argv of the command to run (type=exec); each non-empty
	// line of stdout is treated as an unseal key.
	Command []string `hcl:"command,optional"`
	// TimeoutSeconds bounds how long an exec key source may run. Defaults to 30s.
	TimeoutSeconds int `hcl:"timeout_seconds,optional"`
}

var (
	configFilePath = flag.String("config-file-path", ".", "Directory containing the vault-unsealer config file")
	configFile     = flag.String("config-file", "config.hcl", "Config file name")
)

func newConfig() (*Config, error) {
	flag.Parse()
	return loadConfig(filepath.Join(*configFilePath, *configFile))
}

// loadConfig parses, defaults, and validates an HCL config file.
func loadConfig(path string) (*Config, error) {
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, fmt.Errorf("loading config %q: %w", path, err)
	}
	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.ProbeInterval == 0 {
		c.ProbeInterval = 10
	}
}

func (c *Config) validate() error {
	if len(c.Nodes) == 0 {
		return fmt.Errorf(`config: "nodes" must contain at least one Vault address`)
	}
	if c.ProbeInterval <= 0 {
		return fmt.Errorf(`config: "probe_interval" must be greater than 0`)
	}
	if len(c.KeySources) == 0 {
		return fmt.Errorf(`config: at least one "unseal_key_source" block is required`)
	}
	// The unseal_key_source contents are fully validated when the providers are built.
	return nil
}
