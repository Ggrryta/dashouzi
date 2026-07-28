FROM golang:1.25-alpine

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /server ./cmd/server

ENV TZ=Asia/Shanghai
EXPOSE 8080

CMD ["/server"]
