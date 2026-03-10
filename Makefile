.PHONY: test lint fmt

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

build:
	go build ./...