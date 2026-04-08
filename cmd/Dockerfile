FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /d8x-rpc-proxy ./cmd

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /d8x-rpc-proxy /d8x-rpc-proxy

EXPOSE 8080

ENTRYPOINT ["/d8x-rpc-proxy"]
