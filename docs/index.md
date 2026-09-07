# Vault Unsealer

A tool to implement auto-unsealing of HashiCorp Vault nodes.

Vault Unsealer periodically probes a list of Vault nodes and, whenever a node
reports that it is sealed, submits unseal keys until the node is unsealed again.
It is aimed at **self-hosted Vault Community Edition**, where cloud KMS
auto-unseal (you are not in a cloud) and HSM/PKCS#11 auto-unseal (Vault
Enterprise + an HSM) are not realistic options.

!!! warning "Proof of Concept"
    This is a workflow proof of concept. Auto-unsealing inherently requires
    *something* to hold the unseal keys — read [Security](security.md) and
    evaluate the trade-off carefully before any production use.

## Highlights

- **No secrets in config.** Unseal keys are fetched at runtime from an external
  source and never written to disk by the tool.
- **Pluggable key sources.** Read keys from environment variables or from *any*
  command (`exec`) — 1Password, Nomad Variables, Vault KV, SOPS — with no vendor
  SDKs and no plugin system.
- **Split-custody quorum.** Combine multiple sources so shares can live in
  different places; keys are aggregated until Vault reaches its unseal threshold.
- **Verified TLS** to Vault by default, with per-request timeouts.
- **Keys kept out of swap and core dumps** via `mlock` and core-dump limits.

## Quickstart

Run the container, injecting unseal keys as environment variables (the `env`
key source):

```shell
docker run --rm \
  --cap-add IPC_LOCK --ulimit memlock=-1 \
  -v $(pwd)/config.hcl:/config.hcl:ro \
  -e VAULT_UNSEAL_KEY_1 -e VAULT_UNSEAL_KEY_2 -e VAULT_UNSEAL_KEY_3 \
  ghcr.io/devops-rob/vault-unsealer:latest -config-file-path /
```

```hcl title="config.hcl"
probe_interval = 10
nodes          = ["https://10.0.0.11:8200", "https://10.0.0.12:8200"]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1", "VAULT_UNSEAL_KEY_2", "VAULT_UNSEAL_KEY_3"]
}
```

Next steps:

- [Configuration](configuration.md) — the full config reference and key sources.
- [Security](security.md) — the threat model and hardening guidance.
- [Releasing](releasing.md) — how releases are built, signed, and verified.
