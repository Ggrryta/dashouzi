FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /gateway ./cmd/gateway

FROM alpine:3.18
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /gateway .
COPY --from=builder /app/config ./config
EXPOSE 9000
ENTRYPOINT ["/app/gateway"]
