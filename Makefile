BINARY := k8s-storage-exporter

.PHONY: test lint go-lint timoni-lint build run-dev

test:
	go test ./...

lint: go-lint

go-lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

timoni-lint:
	timoni mod vet ./modules/k8s-storage-exporter

build:
	go build -o $(BINARY) .

# needs $KUBECONFIG, scrapes all nodes
run-dev:
	go run . --disable-exporter-metrics
