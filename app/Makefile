.PHONY: all run seed migrate

BINARY := ./bin/app

all: check lint build

migrate:
	go run -v ./cmd/migrate/

run: migrate
	go run -v ./cmd/api/

check:
	go fmt ./...
	go vet ./...
	go test ./...

lint:
	golangci-lint run

build:
	mkdir -p ./bin
	go build -o $(BINARY) cmd/api/main.go