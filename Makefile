.PHONY: build install clean test run

BIN      := letterhead
CMD      := ./cmd/letterhead
GOBIN    ?= $(shell go env GOPATH)/bin

build:
	go build -o $(BIN) $(CMD)

install:
	go install $(CMD)

clean:
	rm -f $(BIN)

test:
	go test ./...

run:
	go run $(CMD) $(ARGS)
