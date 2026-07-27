.PHONY: run build test clean up down test-integration e2e

# 本地开发
run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

# Docker
up:
	docker compose up -d

down:
	docker compose down

# 测试
test:
	go test ./... -v -count=1

test-cover:
	go test ./... -v -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

# 集成测试（Docker 内运行）
test-integration:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags=integration -o integration.test ./internal/repository/
	docker cp integration.test bloger_app:/integration.test
	docker exec bloger_app /integration.test -test.v
	rm -f integration.test

# E2E 全链路测试
e2e:
	bash ./scripts/e2e_test.sh

# 清理
clean:
	rm -rf bin/ coverage.out integration.test service.test
