# Configuration

Vault Unsealer takes an [HCL](https://github.com/hashicorp/hcl) configuration
file (default `config.hcl`). The configuration file contains **no secrets** —
only where to reach Vault and where to source keys from.

Override the location with `-config-file-path <dir>` and `-config-file <name>`.

## Reference

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `log_level` | string | no | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. Default `info`. |
| `probe_interval` | int | no | Seal-status probe frequency, in seconds. Default `10`. |
| `disable_mlock` | bool | no | Skip locking process memory into RAM. See [Security](security.md#protecting-keys-in-memory). Default `false`. |
| `nodes` | list(string) | yes | Vault node addresses to manage. Use `https://` in production. |
| `tls.ca_cert` | string | no | Path to a PEM CA bundle used to verify Vault's certificate. Defaults to the system trust store. |
| `tls.skip_verify` | bool | no | Disable TLS verification. **Insecure**; local testing only. Default `false`. |
| `unseal_key_source` | block | yes (1+) | Where to fetch unseal keys. Repeatable — see [Multiple sources](#multiple-key-sources). |
| `unseal_key_source.type` | string | yes | `env` or `exec`. |
| `unseal_key_source.env_vars` | list(string) | for `env` | Environment variable names, one unseal key per variable. |
| `unseal_key_source.command` | list(string) | for `exec` | Command argv; each non-empty stdout line is one unseal key. |
| `unseal_key_source.timeout_seconds` | int | no | Timeout for an `exec` command. Default `30`. |

## Key sources

### `env` — keys injected as environment variables

Best when the platform already injects secrets as environment variables (Nomad
`template { env = true }`, systemd `LoadCredential`, `op run`, Kubernetes
secrets).

```hcl
unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1", "VAULT_UNSEAL_KEY_2", "VAULT_UNSEAL_KEY_3"]
}
```

### `exec` — keys from any secret store

`exec` runs a command and reads one unseal key per line from its stdout. This is
the universal adapter: integrate any secret store without a vendor SDK.

```hcl
unseal_key_source {
  type            = "exec"
  command         = ["/etc/vault-unsealer/fetch-keys.sh"]
  timeout_seconds = 30
}
```

Common one-liners for a wrapper script:

=== "1Password"

    ```shell
    op read "op://vault/vault-unseal/key1"
    ```

=== "Nomad Variables"

    ```shell
    nomad var get -item=key1 nomad/jobs/vault-unsealer
    ```

=== "Vault KV"

    ```shell
    vault kv get -field=key1 secret/vault-unseal
    ```

=== "SOPS"

    ```shell
    sops -d --extract '["unseal_keys"]' keys.enc.json | jq -r '.[]'
    ```

### Multiple key sources

Declare more than one `unseal_key_source` block to combine shares that live in
different places. Keys are gathered from **all** sources and submitted until
Vault reports it is unsealed (quorum met).

```hcl
# Share 1: injected by the platform.
unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1"]
}

# Share 2: 1Password.
unseal_key_source {
  type    = "exec"
  command = ["/etc/vault-unsealer/fetch-1password.sh"]
}

# Share 3: Nomad Variables.
unseal_key_source {
  type    = "exec"
  command = ["/etc/vault-unsealer/fetch-nomad.sh"]
}
```

!!! info "Fail-fast vs. tolerant"
    Structurally invalid sources (unknown `type`, `exec` without `command`,
    `env` without `env_vars`) are rejected at **startup**. A source that fails at
    **runtime** (command errors, secret temporarily unavailable) is logged and
    skipped, and the remaining sources are still used — so you can reach quorum
    even when one store is down. Only if *no* source yields a key does the probe
    fail and retry on the next interval. Duplicate keys are de-duplicated.
