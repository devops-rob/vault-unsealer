package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the non-secret runtime configuration for vault-unsealer. Note that
// unseal keys are intentionally NOT part of this struct: they are resolved at
// runtime from an external source (see KeySourceConfig) so that secrets never
// live in the configuration file.
type Config struct {
	LogLevel      string          `mapstructure:"log_level"`
	Nodes         []string        `mapstructure:"nodes"`
	ProbeInterval int             `mapstructure:"probe_interval"`
	TLS           TLSConfig       `mapstructure:"tls"`
	KeySource     KeySourceConfig `mapstructure:"unseal_key_source"`
}

// TLSConfig controls how vault-unsealer connects to Vault's HTTPS API.
type TLSConfig struct {
	// CACert is the path to a PEM bundle used to verify the Vault server
	// certificate. When empty, the system trust store is used.
	CACert string `mapstructure:"ca_cert"`
	// SkipVerify disables TLS certificate verification. This is insecure and
	// should only ever be used for local testing.
	SkipVerify bool `mapstructure:"skip_verify"`
}

// KeySourceConfig selects where unseal keys are fetched from at runtime.
type KeySourceConfig struct {
	// Type is the provider type: "env" or "exec".
	Type string `mapstructure:"type"`
	// EnvVars is the list of environment variable names to read (type=env),
	// one unseal key per variable.
	EnvVars []string `mapstructure:"env_vars"`
	// Command is the argv of the command to run (type=exec); each non-empty
	// line of stdout is treated as an unseal key.
	Command []string `mapstructure:"command"`
	// TimeoutSeconds bounds how long an exec key source may run. Defaults to 30s.
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

var (
	configFilePath = flag.String("config-file-path", ".", "Directory containing the vault-unsealer config file")
	configFile     = flag.String("config-file", "config", "Config file name (without the .json extension)")
)

func newConfig() (*Config, error) {
	flag.Parse()

	name := strings.TrimSuffix(*configFile, ".json")

	v := viper.New()
	v.SetDefault("log_level", "info")
	v.SetDefault("probe_interval", 10)

	v.SetConfigName(name)
	v.SetConfigType("json")
	v.AddConfigPath(*configFilePath)
	v.AddConfigPath("config")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Nodes) == 0 {
		return fmt.Errorf("config: \"nodes\" must contain at least one Vault address")
	}
	if c.ProbeInterval <= 0 {
		return fmt.Errorf("config: \"probe_interval\" must be greater than 0")
	}
	// The unseal_key_source is fully validated when the provider is built.
	return nil
}
