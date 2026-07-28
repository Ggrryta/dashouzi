.PHONY: run build test up down

run:
	go run ./cmd/gateway

build:
	go build -o bin/gateway ./cmd/gateway

test:
	go test ./... -v -count=1

up:
	docker compose up -d

down:
	docker compose down
