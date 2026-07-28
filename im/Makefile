.PHONY: run build test clean up down

build:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/imnode ./cmd/imnode

up:
	docker compose up -d

down:
	docker compose down

test:
	go test ./... -v -count=1

clean:
	rm -rf bin/
