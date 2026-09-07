log_level      = "info"
probe_interval = 10

nodes = ["https://192.168.1.141:8200"]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

# Key shares live in three different places. Vault Unsealer combines all of them
# and submits keys until Vault reaches its unseal threshold (quorum).

# Share 1: injected as an environment variable by the platform.
unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1"]
}

# Share 2: fetched from 1Password.
unseal_key_source {
  type    = "exec"
  command = ["/etc/vault-unsealer/fetch-1password.sh"]
}

# Share 3: fetched from Nomad Variables.
unseal_key_source {
  type    = "exec"
  command = ["/etc/vault-unsealer/fetch-nomad.sh"]
}
