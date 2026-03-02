.PHONY: run build tidy

run:
	CGO_ENABLED=1 go run ./cmd/server

build:
	CGO_ENABLED=1 go build -o bin/server ./cmd/server

tidy:
	go mod tidy
