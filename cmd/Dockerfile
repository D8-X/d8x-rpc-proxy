FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /d8x-rpc-proxy ./cmd

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /d8x-rpc-proxy /d8x-rpc-proxy

EXPOSE 8080

ENTRYPOINT ["/d8x-rpc-proxy"]
