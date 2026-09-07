#!/usr/bin/env bash
#
# Example exec key source for vault-unsealer.
#
# vault-unsealer runs this command whenever a node is found sealed and treats
# each non-empty line printed to stdout as one unseal key. This lets you pull
# keys from ANY secret store without vault-unsealer needing a vendor SDK.
#
# Keys are fetched just-in-time and are never written to disk by vault-unsealer.
# Uncomment ONE of the examples below and adapt it to your environment.

set -euo pipefail

# --- 1Password (op CLI) ---------------------------------------------------
# op read "op://vault/vault-unseal/key1"
# op read "op://vault/vault-unseal/key2"
# op read "op://vault/vault-unseal/key3"

# --- Nomad Variables ------------------------------------------------------
# nomad var get -item=key1 nomad/jobs/vault-unsealer
# nomad var get -item=key2 nomad/jobs/vault-unsealer
# nomad var get -item=key3 nomad/jobs/vault-unsealer

# --- Another Vault (e.g. a KV store you already trust) --------------------
# vault kv get -field=key1 secret/vault-unseal
# vault kv get -field=key2 secret/vault-unseal
# vault kv get -field=key3 secret/vault-unseal

# --- SOPS-encrypted file --------------------------------------------------
# sops -d --extract '["unseal_keys"]' /etc/vault-unsealer/keys.enc.json | jq -r '.[]'

echo "fetch-keys.sh: no key source configured; edit this script" >&2
exit 1
