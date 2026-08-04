.PHONY: up down build logs test test-unit test-integration test-e2e bench-bid bench-concurrent reconcile lint fmt help

# 默认配置文件路径
CONFIG ?= configs/config.yaml

up:
	docker compose -f deployments/docker-compose.yaml up -d --build

down:
	docker compose -f deployments/docker-compose.yaml down -v

build:
	docker compose -f deployments/docker-compose.yaml build

logs:
	docker compose -f deployments/docker-compose.yaml logs -f app

test: test-unit

test-unit:
	go test -race -cover ./...

test-integration:
	docker compose -f deployments/docker-compose.yaml -f deployments/docker-compose.test.yaml up --abort-on-container-exit --build integration-test

test-e2e:
	docker compose -f deployments/docker-compose.yaml -f deployments/docker-compose.test.yaml up --abort-on-container-exit --build e2e-test

bench-bid:
	docker run --rm --network=auction_default williamyeh/wrk \
		-t8 -c1000 -d30s --latency \
		-s /app/test/bench/bid.lua \
		http://app:8080/api/v1/items/1/bids

bench-concurrent:
	go run ./test/bench/concurrent_bid.go

reconcile:
	bash scripts/reconcile.sh

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

help:
	@echo "Available targets:"
	@echo "  up              - Start all services in Docker"
	@echo "  down            - Stop all services and remove volumes"
	@echo "  build           - Build the app image"
	@echo "  logs            - Follow app logs"
	@echo "  test-unit       - Run unit tests locally"
	@echo "  test-integration- Run integration tests in Docker"
	@echo "  test-e2e        - Run E2E tests in Docker"
	@echo "  bench-bid       - Run bid endpoint benchmark with wrk"
	@echo "  bench-concurrent- Run concurrent bid consistency test"
	@echo "  reconcile       - Run data reconciliation script"
	@echo "  lint            - Run linter"
	@echo "  fmt             - Format Go code"
