FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o feed ./cmd/feed

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/feed /usr/local/bin/feed
EXPOSE 8080
CMD ["feed"]
