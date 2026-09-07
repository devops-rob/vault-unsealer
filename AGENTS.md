# AGENTS.md

Operating guidance for AI agents working on `vault-unsealer`.

## What this is

A small Go daemon that periodically probes HashiCorp Vault nodes and unseals any
that are sealed, aimed at self-hosted Vault **Community Edition**. Unseal keys are
sensitive: they are sourced at runtime from an external provider (`env` or
`exec`) and must never be written to disk or committed to the repo.

## Build & checks

Requires Go 1.27+ (`go.mod` pins the toolchain; `GOTOOLCHAIN=auto` will fetch it).

```shell
make build      # go build -o vault-unsealer .
make test       # go test ./...
make vet        # go vet ./...
make vulncheck  # govulncheck ./...
```

If `govulncheck` picks the wrong toolchain, pin it: `GOTOOLCHAIN=go1.27.1 make vulncheck`.

## Testing requirements (default for every change)

Unless the change is trivial (docs, comments, string literals) or the user opts
out, do BOTH of the following before considering a change complete:

1. **Unit tests.** Add or update Go unit tests for any new or changed logic
   (see `keyprovider_test.go`, `unseal_test.go` for the style), then run
   `make test`, `make vet`, and `make vulncheck` — all must pass.

2. **Real-Vault end-to-end check.** Prove the tool actually unseals a real,
   sealed Vault. Do not rely on unit tests alone. Outline:
   - Install the `vault` CLI if missing (download from
     `https://releases.hashicorp.com/vault/`).
   - Start a file-backed Vault server (not `-dev`, which starts unsealed). For
     transport coverage, use a TLS listener with a self-signed cert whose SAN
     includes the node IP (e.g. `openssl req -x509 ... -addext
     "subjectAltName=IP:127.0.0.1"`).
   - `vault operator init -key-shares=3 -key-threshold=2 -format=json` and keep
     the returned keys out of any committed file.
   - Write a `config.json` (no secrets) pointing at the node with
     `tls.ca_cert` set, and an `unseal_key_source` of `env` and/or `exec`.
   - Run the built binary and confirm the node goes `Sealed true -> false`.
   - Also confirm auto-recovery: `vault operator seal` (root token) and verify
     the running daemon re-unseals within one `probe_interval`.
   - When touching TLS code, also confirm verification is enforced: without
     `tls.ca_cert` the self-signed node must be rejected
     (`x509: certificate signed by unknown authority`).

Run long-lived processes (Vault server, the unsealer daemon) in tmux sessions so
they can be inspected and left running for the user.

## Conventions

- Never put unseal keys, tokens, or other secrets in `config.json`, committed
  files, logs, or test fixtures. The config file is secret-free by design.
- Keep HTTPS certificate verification on by default; `tls.skip_verify` is a
  local-testing escape hatch only.
- The `.gitignore`d compiled `vault-unsealer` binary must not be committed.
