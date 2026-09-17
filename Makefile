BIN := session-top

.PHONY: all build test vet fmt clean install

all: test build

build:
	go build -o $(BIN) ./cmd/session-top

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f $(BIN)

install:
	go install ./cmd/session-top
