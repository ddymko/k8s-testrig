PREFIX ?= "/usr/local/bin"
GOOS ?= $(shell go env | grep GOOS | awk -F'=' '{ print $$2 }')
GOPATH ?= $(shell go env | grep GOPATH | awk -F '=' '{ print $$2 }')
GOVERSION=1.11

.PHONY: build
build: ## Build binary
	go build

.PHONY: install
install: build ## Install binary
	cp ./testrig $(PREFIX)/testrig

.PHONY: docker-build
docker-build: ## Build binary using docker
	docker run -it --rm -v $(GOPATH)/pkg/mod:/go/pkg/mod -v $(PWD):/tmp/testrig -w /tmp/testrig -e GOOS=$(GOOS) golang:$(GOVERSION) make build