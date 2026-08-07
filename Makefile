.PHONY: help test vet lint check build install clean default all

.DEFAULT_GOAL := default

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_.-]+:.*## / {printf "%-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run the test suite
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

check: test vet ## Run tests and vet

build: ## Build the gommit CLI
	go build -o gommit .

install: ## Install gommit into Go's bin directory
	go install .

clean: ## Remove local build and test outputs
	rm -f gommit

default: build ## Build gommit

all: clean check build install ## Clean, check, build, and install
