# Vaulter Unsealer

A tool to implement auto-unsealing of HashiCorp Vault nodes. 

_**NOTE: This is designed as a workflow Proof of Concept. Production use of Vault Unsealer is discouraged at present.**_

# Configuration

Vault Unsealer takes a `.json` configuration file with the following configuration parameters:

- `log_level` _(type: string, required: false)_ - This sets the servers log level output. Supported values are `trace`, `debug`, `info`, `warn`, and `err`. The default log level is `info`.
- `probe_interval` _(type: int, required: true)_ - This specifies the frequency of the Vault seal status probe check in seconds.
- `nodes` _(type: []string, required: true)_ - This is a list of Vault server nodes that Vault Unsealer will manage the seal status of.
- `unseal_keys` _(type: []string, required: true)_ - A list of Vault unseal keys that can be used to unseal Vault. The number of keys in this list should be equal to or greater than the unseal threshold required for your Vault cluster.

**Example Configuration**

```json
{
  "log_level": "debug",
  "probe_interval": 10,
  "nodes": [
    "http://192.168.1.141:8200",
    "http://192.168.1.142:8200",
    "http://192.168.1.143:8200"
  ],
  "unseal_keys": [
    "aa109356340az6f2916894c2e538f7450412056cea4c45b3dd4ae1f9c840befc1a",
    "4948bcfe36834c8e6861f8144672cb804610967c7afb0588cfd03217b4354a8c35",
    "7b5802f21b19s522444e2723a31cb07d5a3de60fbc37d21f918f998018b6e7ce8b"
  ]
}
```

# Usage

### Docker

```shell
docker run -v $(pwd)/example.json:/app/config.json \
  devopsrob/vault-unsealer:0.1 /app/vault-unsealer
```

### Nomad (Docker Job)

This example stores the unseal keys in encrypted Nomad variables and uses Nomad templating to render the config file.

```hcl
job "vault-unsealer" {
  namespace   = "vault-cluster"
  datacenters = ["dc1"]
  type        = "service"
  node_pool   = "vault-servers"

  group "vault-unsealer" {
    count = 1

    constraint {
      attribute = "${node.class}"
      value     = "vault-servers"
    }

    task "vault-unsealer" {
      driver = "docker"

      config {
        image      = "devopsrob/vault-unsealer:0.2"


        command = "./vault-unsealer"
        volumes = [
          "local/config:/app/config"
        ]
      }

      template {
        data = <<EOH

{
  "log_level": "debug",
  "probe_interval": 10,
  "nodes": [
{{- $nodes := nomadService "vault" }}
{{- range $i, $e := $nodes }}
    {{- if $i }},{{ end }}
    "http://{{ .Address }}:{{ .Port }}"
{{- end }}
  ],
  "unseal_keys": [
    {{- with nomadVar "nomad/jobs/vault-unsealer" }}
    "{{ .key1 }}"
    , "{{ .key2 }}"
    , "{{ .key3 }}"
    {{- end }}
  ]
}
EOH

        destination = "local/config/config.json"
        change_mode = "noop"
      }

      resources {
        cpu    = 100
        memory = 512

      }

      affinity {
        attribute = "${meta.node_id}"
        value     = "${NOMAD_ALLOC_ID}"
        weight    = 100
      }
    }
  }
}
```

# Best Practice Tip

Vault unsealer requires unseal keys which are highly sensitive pieces of data. It is recommended that the config file is rendered with the unseal keys values coming from an encrypted store that you trust. The Nomad job usage above is an example of how to achieve this.