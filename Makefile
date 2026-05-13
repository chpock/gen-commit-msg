.DEFAULT_GOAL := help

.PHONY: help build test vet lint fmt clean all

BINARY := gen-commit-msg
CMD := ./cmd/$(BINARY)
VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

help: ## Show this help
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

VERBOSE ?=

test: ## Run tests (VERBOSE=1 for verbose output)
	go test -count=1 -race $(if $(VERBOSE),-v) ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Run go fmt
	go fmt ./...

clean: ## Remove the binary
	rm -f $(BINARY)

all: fmt vet lint test build ## Run all checks and build
