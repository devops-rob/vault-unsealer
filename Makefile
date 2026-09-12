service_name = vault-unsealer
org = devopsrob
# Version is derived from git for local/manual builds; releases are driven by
# git tags via GoReleaser (see .goreleaser.yaml and .github/workflows/release.yml).
version = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet tidy vulncheck snapshot release-check docker_build tag push jp_push deploy

build:
	go build -o $(service_name) .

test:
	go test ./... -v

vet:
	go vet ./...

tidy:
	go mod tidy

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Validate the GoReleaser configuration.
release-check:
	go run github.com/goreleaser/goreleaser/v2@latest check

# Build a local release (binaries + archives) without publishing anything.
snapshot:
	go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean

# --- Manual Docker publishing (CI normally does this on tag) ---------------
docker_build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(org)/$(service_name):$(version) . --push

tag:
	docker tag $(org)/$(service_name):$(version) $(org)/$(service_name):$(version)

push:
	docker push $(org)/$(service_name):$(version)

jp_push:
	jumppad push $(org)/$(service_name):$(version) resource.nomad_cluster.dev

deploy: docker_build tag push
