# Vault Unsealer

A tool to implement auto-unsealing of HashiCorp Vault nodes.

It periodically probes a list of Vault nodes and, whenever a node reports that it
is sealed, submits unseal keys until the node is unsealed again. It is aimed at
**self-hosted Vault Community Edition**, where cloud KMS auto-unseal (you are not
in a cloud) and HSM/PKCS#11 auto-unseal (Vault Enterprise + an HSM) are not
realistic options.

## Security model — read this first

Auto-unsealing an on-prem Vault Community cluster fundamentally requires
*something* to hold the unseal keys and present them to Vault. Vault Unsealer is
that something, so it is important to understand the trade-off:

- To unseal, Vault Unsealer must be able to obtain **at least the unseal
  threshold** of key shares. Co-locating that many shares in one place
  intentionally works against Shamir's split-trust design: whoever can compromise
  the unsealer (its host, its secret source, its memory) can unseal Vault.
- Vault Unsealer minimises that exposure. It **never stores unseal keys in its
  config file** and never writes them to disk. Keys are fetched from an external
  source **only when a node is actually found sealed**, used for that unseal
  attempt, and then discarded.
- All communication with Vault uses **HTTPS with certificate verification** by
  default.

If you *can* use a stronger option, prefer it:

- **Transit auto-unseal** works on Vault **Community Edition**: one Vault can
  auto-unseal other Vaults via the `transit` seal — no Enterprise, HSM, or cloud
  required. It doesn't remove the problem entirely (the "unseal Vault" that holds
  the transit key must itself be unsealed by someone), but it lets you shrink the
  blast radius to a single root-of-trust Vault, which is exactly where a tool like
  this one is still useful.
- **Cloud KMS / HSM auto-unseal** removes the need to hold keys at all, if your
  environment supports them.

> **NOTE: This is a workflow Proof of Concept. Evaluate the security trade-off
> above carefully before any production use.**

### Protecting keys in memory

While Vault Unsealer holds unseal keys (only during an unseal attempt), those
bytes could otherwise be written to disk via swap or a crash core dump. Two
layers guard against that:

**Built in (automatic):**

- **Memory locking (`mlock`).** On Linux the process locks its pages into RAM
  (`mlockall`) so keys cannot be paged out to swap. This needs the `IPC_LOCK`
  capability (or root) and a sufficient `RLIMIT_MEMLOCK`. If it can't (missing
  capability), the tool logs a warning and continues — set `disable_mlock = true`
  to skip the attempt and silence the warning when you mitigate swap another way.
- **No core dumps.** Core dumps are disabled at startup (`RLIMIT_CORE=0` and
  `PR_SET_DUMPABLE=0`), which also blocks unprivileged `ptrace` reads of the
  process memory.

To grant the capability:

```shell
# Docker
docker run --cap-add IPC_LOCK --ulimit memlock=-1 ... devopsrob/vault-unsealer:0.3

# Nomad task config: cap_add = ["IPC_LOCK"]
```

**Host hardening (recommended, and what HashiCorp advise for Vault too):**

- **Disable swap** on the unsealer host (`swapoff -a`), or use **encrypted swap**
  (e.g. dm-crypt with a random key) so anything paged out is encrypted at rest.
- Note: `mlock` prevents swapping but does not protect against a root attacker,
  `/proc/<pid>/mem`, or a live debugger. Treat the unsealer host as sensitive.

## Configuration

Vault Unsealer takes an [HCL](https://github.com/hashicorp/hcl) configuration
file (default `config.hcl`). The configuration file contains **no secrets** —
only where to reach Vault and where to source keys from.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `log_level` | string | no | Log level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. Default `info`. |
| `probe_interval` | int | no | Seal-status probe frequency, in seconds. Default `10`. |
| `disable_mlock` | bool | no | Skip locking process memory into RAM. Locking is on by default to keep keys out of swap; see [Protecting keys in memory](#protecting-keys-in-memory). Default `false`. |
| `nodes` | []string | yes | Vault node addresses to manage. Use `https://` in production. |
| `tls.ca_cert` | string | no | Path to a PEM CA bundle used to verify Vault's TLS certificate. Defaults to the system trust store. |
| `tls.skip_verify` | bool | no | Disable TLS verification. **Insecure**; local testing only. Default `false`. |
| `unseal_key_source.type` | string | yes | Where to fetch unseal keys: `env` or `exec`. |
| `unseal_key_source.env_vars` | []string | for `env` | Environment variable names, one unseal key per variable. |
| `unseal_key_source.command` | []string | for `exec` | Command argv to run; each non-empty stdout line is treated as one unseal key. |
| `unseal_key_source.timeout_seconds` | int | no | Timeout for the `exec` command. Default `30`. |

### Key sources

The `unseal_key_source` block may be repeated to combine shares from several
places (see [Multiple key sources](#multiple-key-sources-split-custody-quorum)).

#### `env` — keys injected as environment variables

Best when the surrounding platform already injects secrets as environment
variables (Nomad template with `env = true`, systemd `LoadCredential`,
`op run`, Kubernetes secrets, ...).

```hcl
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

#### `exec` — keys from any secret store

`exec` runs a command of your choosing and reads one unseal key per line from its
stdout. This is the universal adapter: integrate any secret store without Vault
Unsealer depending on a vendor SDK. See [`examples/fetch-keys.sh`](examples/fetch-keys.sh)
for 1Password, Nomad Variables, Vault KV, and SOPS recipes.

```hcl
probe_interval = 10
nodes          = ["https://10.0.0.11:8200"]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

unseal_key_source {
  type            = "exec"
  command         = ["/etc/vault-unsealer/fetch-keys.sh"]
  timeout_seconds = 30
}
```

A few one-liners you can drop into a wrapper script:

```shell
# 1Password
op read "op://vault/vault-unseal/key1"

# Nomad Variables
nomad var get -item=key1 nomad/jobs/vault-unsealer

# SOPS-encrypted file
sops -d --extract '["unseal_keys"]' keys.enc.json | jq -r '.[]'
```

#### Multiple key sources (split-custody quorum)

You can declare **more than one** `unseal_key_source` block. This is useful when
the key shares genuinely live in different places — for example one share
injected as an environment variable, another in 1Password, and another in Nomad
Variables. Vault Unsealer gathers keys from **all** configured sources and keeps
submitting them until Vault reports it is unsealed (quorum met).

```hcl
nodes = ["https://10.0.0.11:8200"]

tls {
  ca_cert = "/etc/vault-unsealer/ca.pem"
}

# Share 1: injected into the environment by the platform.
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
```

Behavior:

- **Fail-fast on misconfiguration:** a structurally invalid block (unknown
  `type`, `exec` with no `command`, `env` with no `env_vars`) is rejected at
  startup.
- **Tolerant at runtime:** if one source is temporarily unavailable (a command
  errors, a secret can't be read), it is logged and skipped, and the remaining
  sources are still used. Only if *no* source yields any key does the probe fail
  and retry on the next interval.
- Duplicate keys returned by multiple sources are de-duplicated.

## Usage

Vault Unsealer looks for a file named `config.hcl` in the current directory by
default. Override with `-config-file-path <dir>` and `-config-file <name>`.

### Docker

The published image is a minimal, non-root distroless image. Inject keys as
environment variables (the `env` source):

```shell
docker run --rm \
  --cap-add IPC_LOCK --ulimit memlock=-1 \
  -v $(pwd)/config.hcl:/config.hcl:ro \
  -e VAULT_UNSEAL_KEY_1 -e VAULT_UNSEAL_KEY_2 -e VAULT_UNSEAL_KEY_3 \
  devopsrob/vault-unsealer:0.3 -config-file-path /
```

`--cap-add IPC_LOCK --ulimit memlock=-1` lets the container lock keys out of
swap (see [Protecting keys in memory](#protecting-keys-in-memory)); omit them
only if you set `disable_mlock = true`.

> The `exec` source requires the referenced binaries (e.g. `op`, `nomad`) to be
> present in the runtime image or host. In containers the `env` source is usually
> the better fit.

### Nomad (Docker Job)

This example stores the unseal keys in encrypted Nomad Variables and renders them
into the task's **environment** (not into a file), then uses the `env` key source.

```hcl
job "vault-unsealer" {
  namespace   = "vault-cluster"
  datacenters = ["dc1"]
  type        = "service"
  node_pool   = "vault-servers"

  group "vault-unsealer" {
    count = 1

    task "vault-unsealer" {
      driver = "docker"

      config {
        image   = "devopsrob/vault-unsealer:0.3"
        command = "-config-file-path"
        args    = ["/local"]
        volumes = ["local/config.hcl:/local/config.hcl"]
        cap_add = ["IPC_LOCK"] # lock unseal keys out of swap
      }

      template {
        destination = "local/config.hcl"
        change_mode = "restart"
        data        = <<EOH
log_level      = "info"
probe_interval = 10
nodes = [
{{- $nodes := nomadService "vault" }}
{{- range $i, $e := $nodes }}
  {{- if $i }},{{ end }}
  "https://{{ .Address }}:{{ .Port }}"
{{- end }}
]
tls {
  ca_cert = "/local/ca.pem"
}
unseal_key_source {
  type     = "env"
  env_vars = ["VAULT_UNSEAL_KEY_1", "VAULT_UNSEAL_KEY_2", "VAULT_UNSEAL_KEY_3"]
}
EOH
      }

      # Render the unseal keys straight into the environment from an encrypted
      # Nomad Variable; they never land in a file.
      template {
        destination = "secrets/keys.env"
        env         = true
        change_mode = "restart"
        data        = <<EOH
{{- with nomadVar "nomad/jobs/vault-unsealer" }}
VAULT_UNSEAL_KEY_1={{ .key1 }}
VAULT_UNSEAL_KEY_2={{ .key2 }}
VAULT_UNSEAL_KEY_3={{ .key3 }}
{{- end }}
EOH
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
```

## Development

Requires Go 1.27+.

```shell
make build         # build the vault-unsealer binary
make test          # run unit tests
make vet           # go vet
make vulncheck     # govulncheck vulnerability scan
make release-check # validate the GoReleaser config
make snapshot      # build a local release (binaries + archives) without publishing
```

Continuous integration (`.github/workflows/ci.yml`) runs vet, build, tests, and
`govulncheck` on every push and pull request to `main`.

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com) and GitHub
Actions (`.github/workflows/release.yml`), triggered by pushing a semver tag:

```shell
git tag v0.4.0
git push origin v0.4.0
```

On that tag the workflow runs the test suite and then:

- builds `linux`/`darwin` binaries for `amd64`/`arm64`, packages them as
  `.tar.gz` archives with `checksums.txt`, and publishes a GitHub Release with an
  auto-generated changelog;
- builds and pushes a multi-arch (`linux/amd64,linux/arm64`) container image,
  tagged with the full version, `major.minor`, and `latest`.

### Container registries and secrets

- **GHCR** (`ghcr.io/<owner>/vault-unsealer`) is always published using the
  built-in `GITHUB_TOKEN` — no setup required.
- **Docker Hub** (`docker.io/devopsrob/vault-unsealer`) is published only when
  the following repository secrets are configured (otherwise it is skipped):
  - `DOCKERHUB_USERNAME`
  - `DOCKERHUB_TOKEN` (a Docker Hub access token)

Preview the whole build locally without publishing anything with `make snapshot`
(artifacts land in `dist/`).
