log_level      = "info"
probe_interval = 10

nodes = [
  "https://192.168.1.141:8200",
  "https://192.168.1.142:8200",
  "https://192.168.1.143:8200",
]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

unseal_key_source {
  type = "env"
  env_vars = [
    "VAULT_UNSEAL_KEY_1",
    "VAULT_UNSEAL_KEY_2",
    "VAULT_UNSEAL_KEY_3",
  ]
}
