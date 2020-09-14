# builder for backend
FROM golang:1.15.2-alpine AS builder

WORKDIR /app

COPY main.go go.mod go.sum ./
COPY ./vendor ./vendor
COPY ./internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags "-s -w" -o ./bin/hooker main.go

# target
FROM alpine:3.12

WORKDIR /app
COPY --from=builder /app/bin .

ENV PORT=80

EXPOSE 80

CMD ["./hooker"]