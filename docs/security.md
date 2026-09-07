# Security

## Security model — read this first

Auto-unsealing an on-prem Vault Community cluster fundamentally requires
*something* to hold the unseal keys and present them to Vault. Vault Unsealer is
that something, so understand the trade-off:

- To unseal, Vault Unsealer must be able to obtain **at least the unseal
  threshold** of key shares. Co-locating that many shares in one place
  intentionally works against Shamir's split-trust design: whoever compromises
  the unsealer (its host, its secret source, its memory) can unseal Vault.
- Vault Unsealer minimises that exposure. It **never stores unseal keys in its
  config file** and never writes them to disk. Keys are fetched from an external
  source **only when a node is found sealed**, used for that attempt, and
  discarded.
- All communication with Vault uses **HTTPS with certificate verification** by
  default.

!!! tip "Prefer a stronger option when you can"
    - **Transit auto-unseal** works on Vault **Community Edition**: one Vault can
      auto-unseal others via the `transit` seal — no Enterprise, HSM, or cloud.
      It doesn't remove the problem (the "unseal Vault" holding the transit key
      must itself be unsealed), but it shrinks the blast radius to a single
      root-of-trust Vault, which is exactly where a tool like this stays useful.
    - **Cloud KMS / HSM auto-unseal** removes the need to hold keys at all, if
      your environment supports them.

## Transport security

Vault Unsealer talks to Vault over HTTPS with certificate verification enabled
by default. Provide a private CA with `tls.ca_cert`; the system trust store is
used otherwise. `tls.skip_verify` exists for local testing only and logs a
warning when enabled. Every request is bounded by a timeout so a hung node
cannot stall the probe loop.

## Protecting keys in memory

While Vault Unsealer holds unseal keys (only during an unseal attempt), those
bytes could otherwise reach disk via swap or a crash core dump. Two layers guard
against that.

**Built in (automatic):**

- **Memory locking (`mlock`).** On Linux the process locks its pages into RAM
  (`mlockall`) so keys cannot be paged to swap. This needs the `IPC_LOCK`
  capability (or root) and a sufficient `RLIMIT_MEMLOCK`. If it can't, the tool
  logs a warning and continues; set `disable_mlock = true` to skip the attempt
  when you mitigate swap another way.
- **No core dumps.** Core dumps are disabled at startup (`RLIMIT_CORE=0` and
  `PR_SET_DUMPABLE=0`), which also blocks unprivileged `ptrace` reads.

Grant the capability:

=== "Docker"

    ```shell
    docker run --cap-add IPC_LOCK --ulimit memlock=-1 ... \
      ghcr.io/devops-rob/vault-unsealer:latest
    ```

=== "Nomad"

    ```hcl
    config {
      cap_add = ["IPC_LOCK"]
    }
    ```

**Host hardening (recommended, and what HashiCorp advise for Vault too):**

- **Disable swap** (`swapoff -a`) or use **encrypted swap** (e.g. dm-crypt with a
  random key) so anything paged out is encrypted at rest.
- `mlock` prevents swapping but does not protect against a root attacker,
  `/proc/<pid>/mem`, or a live debugger. Treat the unsealer host as sensitive.
