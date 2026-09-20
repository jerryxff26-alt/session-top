BIN := session-top
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.2)
LDFLAGS := -X github.com/jerryxff26-alt/session-top/internal/cli.Version=$(VERSION)

.PHONY: all build test vet fmt clean install

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/session-top

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f $(BIN)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/session-top
