.PHONY: build test vet lint fmt clean all

BINARY := gen-commit-msg
CMD := ./cmd/$(BINARY)
VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test -count=1 -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY)

all: fmt vet test build
