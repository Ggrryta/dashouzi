.PHONY: run build test clean up down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -v -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	rm -rf bin/ coverage.out
