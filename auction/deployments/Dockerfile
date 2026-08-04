FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/auction ./cmd/auction

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=builder /app/auction /app/auction
COPY --from=builder /app/configs /app/configs

EXPOSE 8080

CMD ["/app/auction"]
