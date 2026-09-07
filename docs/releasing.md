# Releasing

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

## Container registries and secrets

- **GHCR** (`ghcr.io/devops-rob/vault-unsealer`) is always published using the
  built-in `GITHUB_TOKEN` — no setup required.
- **Docker Hub** (`docker.io/devopsrob/vault-unsealer`) is published only when
  the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` are set;
  otherwise it is skipped.

## Supply-chain security

Every release ships verifiable provenance:

- **SBOMs** — an SPDX SBOM per archive; the container image carries SBOM +
  max-mode provenance attestations.
- **Signatures** — `checksums.txt` and the image are signed with
  [cosign](https://docs.sigstore.dev/) using keyless (Sigstore/OIDC) signing.

Verify a downloaded release:

```shell
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp '^https://github.com/devops-rob/vault-unsealer' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

Verify the image:

```shell
cosign verify \
  --certificate-identity-regexp '^https://github.com/devops-rob/vault-unsealer' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/devops-rob/vault-unsealer:<tag>
```

## Local preview

Build the whole thing locally without publishing anything:

```shell
make release-check   # validate the GoReleaser config
make snapshot        # build binaries + archives + SBOMs into dist/
```
