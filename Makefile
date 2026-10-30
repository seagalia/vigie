GINKGO = go tool ginkgo

# the release binary is built without cgo, so nothing else relies on it either
export CGO_ENABLED = 0

.PHONY: build test test-linux test-e2e lint docker generate api-lint

build:
	go build -o vigie .

# unit tests on the current platform.
# cgo is only enabled here because the race detector needs it on linux.
test:
	CGO_ENABLED=1 $(GINKGO) --randomize-all --race --label-filter='!e2e' -r

# unit tests on linux, as in production.
# cgo is only enabled here because the race detector needs it on linux.
test-linux:
	docker run --rm \
		-v $(CURDIR):/src/vigie \
		-v vigie-gomod:/go/pkg/mod \
		-v vigie-gocache:/root/.cache/go-build \
		-w /src/vigie \
		-e CGO_ENABLED=1 \
		golang:1.27 $(GINKGO) --randomize-all --race --label-filter='!e2e' -r

# the kubernetes and dns sources against a kind cluster and a consul agent, needs docker.
# testcontainers ignores docker contexts (colima, docker desktop), so the socket of the current context is passed on,
# and the containers it starts mount the socket as seen from inside the docker vm.
test-e2e:
	DOCKER_HOST=$${DOCKER_HOST:-$$(docker context inspect --format '{{.Endpoints.docker.Host}}')} \
	TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=$${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-/var/run/docker.sock} \
	$(GINKGO) --label-filter=e2e -v ./e2e

lint:
	golangci-lint run ./...

# the vigie image
docker:
	docker build --build-arg VERSION=$$(git describe --tags --always) --build-arg COMMIT=$$(git rev-parse HEAD) --build-arg DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) -t vigie:dev .

# regenerate the go code of the protos in api/.
# outputs are removed first, so a deleted proto leaves nothing behind.
generate:
	rm -rf sdk/vigie
	cd api && buf generate

api-lint:
	cd api && buf lint
