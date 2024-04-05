service_name = "vault-unsealer"
version = "0.3"
org = "devopsrob"

go_test:
	go test ./src -v
go_build:
	go build ./src -o $(service_name)
docker_build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(org)/$(service_name):$(version) . --push
tag:
	docker tag  $(org)/$(service_name):$(version) $(org)/$(service_name):$(version)
push:
	docker push  $(org)/$(service_name):$(version)
jp_push:
	jumppad push $(org)/$(service_name):$(version) resource.nomad_cluster.dev
deploy: docker_build tag push