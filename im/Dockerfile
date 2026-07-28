FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /imnode ./cmd/imnode

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /gateway .
COPY --from=builder /imnode .
COPY --from=builder /app/config ./config

EXPOSE 8080 8081 8082 8083

ENTRYPOINT ["/app/gateway"]
CMD ["-config", "config/gateway.yaml"]
