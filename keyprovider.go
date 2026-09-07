package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"
)

// KeyProvider retrieves Vault unseal keys from an external source at runtime.
//
// Keys are fetched on demand (only when a node is actually found sealed) and are
// never written to disk by vault-unsealer. This keeps the sensitive unseal keys
// out of the configuration file and out of long-lived process state.
type KeyProvider interface {
	UnsealKeys(ctx context.Context) ([]string, error)
}

// envKeyProvider reads one unseal key from each named environment variable.
// The keys themselves are expected to be injected by the surrounding platform
// (Nomad template with env = true, systemd LoadCredential, `op run`, Kubernetes
// secrets, ...), so they never touch disk in plaintext.
type envKeyProvider struct {
	vars []string
}

func (p *envKeyProvider) UnsealKeys(_ context.Context) ([]string, error) {
	keys := make([]string, 0, len(p.vars))
	for _, name := range p.vars {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			return nil, fmt.Errorf("environment variable %q is empty or unset", name)
		}
		keys = append(keys, v)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no unseal keys resolved from environment")
	}
	return keys, nil
}

// execKeyProvider runs an external command and treats each non-empty line of its
// stdout as an unseal key. This is the universal adapter: it lets operators
// integrate any secret store (1Password `op read`, `nomad var get`, `vault kv
// get`, `sops -d`, cloud CLIs, ...) without vault-unsealer depending on any
// particular vendor SDK. Write a small wrapper script if you need to combine
// several sources into the required number of key shares.
type execKeyProvider struct {
	command []string
	timeout time.Duration
}

func (p *execKeyProvider) UnsealKeys(ctx context.Context) ([]string, error) {
	if len(p.command) == 0 {
		return nil, fmt.Errorf("exec key source: no command configured")
	}
	if p.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, p.command[0], p.command[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("exec key source command failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	keys := parseKeys(stdout.String())
	if len(keys) == 0 {
		return nil, fmt.Errorf("exec key source produced no unseal keys on stdout")
	}
	return keys, nil
}

// parseKeys splits command/file style output into individual, trimmed,
// non-empty keys, one per line.
func parseKeys(out string) []string {
	var keys []string
	for _, line := range strings.Split(out, "\n") {
		if k := strings.TrimSpace(line); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// multiKeyProvider aggregates unseal keys from several underlying providers. It
// lets keys genuinely live in different places (e.g. one env-injected share plus
// two shares fetched by different commands) and combines them so the unseal loop
// can keep submitting keys until Vault reports quorum.
//
// It is deliberately tolerant at fetch time: if one source fails (a command
// errors, a secret is temporarily unavailable, ...) the others are still used.
// It only errors when NO source yields any key. Structural misconfiguration is
// caught earlier, when the providers are built.
type multiKeyProvider struct {
	providers []labeledProvider
}

type labeledProvider struct {
	label    string
	provider KeyProvider
}

func (m *multiKeyProvider) UnsealKeys(ctx context.Context) ([]string, error) {
	var all []string
	seen := make(map[string]struct{})

	for _, lp := range m.providers {
		keys, err := lp.provider.UnsealKeys(ctx)
		if err != nil {
			logger.Warnf("unseal key source %s failed, continuing with other sources: %v", lp.label, err)
			continue
		}
		added := 0
		for _, k := range keys {
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			all = append(all, k)
			added++
		}
		logger.Debugf("unseal key source %s provided %d key(s) (%d new)", lp.label, len(keys), added)
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("no unseal keys could be retrieved from any of the %d configured sources", len(m.providers))
	}
	return all, nil
}

// newKeyProviders builds a KeyProvider from one or more source configurations.
// Every source is validated up front; a structural error in any source is fatal
// so misconfiguration is caught at startup rather than during an outage.
func newKeyProviders(cfgs []KeySourceConfig) (KeyProvider, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one unseal_key_source is required")
	}
	providers := make([]labeledProvider, 0, len(cfgs))
	for i, cfg := range cfgs {
		p, err := newKeyProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("unseal_key_source #%d: %w", i+1, err)
		}
		providers = append(providers, labeledProvider{label: keySourceLabel(i, cfg), provider: p})
	}
	if len(providers) == 1 {
		// A single source needs no aggregation wrapper.
		return providers[0].provider, nil
	}
	return &multiKeyProvider{providers: providers}, nil
}

// keySourceLabel builds a short, non-secret label for logging.
func keySourceLabel(index int, cfg KeySourceConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "exec":
		if len(cfg.Command) > 0 {
			return fmt.Sprintf("#%d exec[%s]", index+1, cfg.Command[0])
		}
		return fmt.Sprintf("#%d exec", index+1)
	case "env":
		return fmt.Sprintf("#%d env%v", index+1, cfg.EnvVars)
	default:
		return fmt.Sprintf("#%d %s", index+1, cfg.Type)
	}
}

const defaultExecTimeout = 30 * time.Second

// newKeyProvider builds a KeyProvider from configuration. The plaintext
// keys-in-config-file source from the original proof of concept has been
// removed: keys must now come from an external source (env or exec).
func newKeyProvider(cfg KeySourceConfig) (KeyProvider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "env":
		if len(cfg.EnvVars) == 0 {
			return nil, fmt.Errorf(`unseal_key_source.type="env" requires a non-empty "env_vars" list`)
		}
		return &envKeyProvider{vars: cfg.EnvVars}, nil
	case "exec":
		if len(cfg.Command) == 0 {
			return nil, fmt.Errorf(`unseal_key_source.type="exec" requires a non-empty "command"`)
		}
		timeout := defaultExecTimeout
		if cfg.TimeoutSeconds > 0 {
			timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
		}
		return &execKeyProvider{command: cfg.Command, timeout: timeout}, nil
	case "":
		return nil, fmt.Errorf(`unseal_key_source.type must be set (supported: "env", "exec")`)
	default:
		return nil, fmt.Errorf("unsupported unseal_key_source.type %q (supported: env, exec)", cfg.Type)
	}
}
