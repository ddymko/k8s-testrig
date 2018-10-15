PREFIX ?= /usr/local/bin
GOOS ?= $(shell go env | grep GOOS | awk -F'=' '{ print $$2 }')
GOPATH ?= $(shell go env | grep GOPATH | awk -F '=' '{ print $$2 }')
GOVERSION=1.11

# Make sure we don't create the pkg/mod dir if it doesn't exist
# The docker `--mount` flag ensures that it won't be created, but we also don't want to error out if it is missing
GOMODCACHE_MOUNT := $(shell [ -d $(GOPATH)/pkg/mod ] && echo \--mount type=bind,source=$(GOPATH)/pkg/mod,target=/go/pkg/mod)

all: build install

clean:
	rm bin/*

.PHONY: build
build: ## Build binary
	GOMODULES=1 go build -o bin/testrig

.PHONY: install
install: ## Install binary
	cp bin/testrig $(PREFIX)/testrig

.PHONY: docker-build
docker-build: ## Build binary using docker
	docker run -it --rm $(GOMODCACHE_MOUNT) -v $(PWD):/tmp/testrig -w /tmp/testrig -e GOOS=$(GOOS) golang:$(GOVERSION) make build
