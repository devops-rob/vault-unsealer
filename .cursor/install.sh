#!/usr/bin/env bash
set -euo pipefail

# Repository bootstrap for the vault-unsealer Cloud Agent environment.
# Idempotent: safe to run repeatedly against cached state.

cd "$(dirname "$0")/.."

# Download Go module dependencies and build the vault-unsealer binary.
go mod download
go build -o vault-unsealer .

# Best-effort install of the Vault CLI so agents can run vault-unsealer
# end-to-end against a local sealed Vault. Non-fatal if it cannot be fetched.
VAULT_VERSION="1.15.6"
if ! command -v vault >/dev/null 2>&1; then
  tmp="$(mktemp -d)"
  if curl -fsSL "https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip" -o "${tmp}/vault.zip" \
    && unzip -o -q "${tmp}/vault.zip" -d "${tmp}"; then
    sudo mv "${tmp}/vault" /usr/local/bin/vault
  else
    echo "warning: could not install Vault CLI (network?); skipping" >&2
  fi
  rm -rf "${tmp}"
fi

echo "vault-unsealer install complete: built $(ls -la vault-unsealer | awk '{print $5}') byte binary"
