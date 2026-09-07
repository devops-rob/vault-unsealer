service_name = vault-unsealer
version = 0.3
org = devopsrob

.PHONY: build test vet tidy vulncheck docker_build tag push jp_push deploy

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

docker_build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(org)/$(service_name):$(version) . --push

tag:
	docker tag $(org)/$(service_name):$(version) $(org)/$(service_name):$(version)

push:
	docker push $(org)/$(service_name):$(version)

jp_push:
	jumppad push $(org)/$(service_name):$(version) resource.nomad_cluster.dev

deploy: docker_build tag push
