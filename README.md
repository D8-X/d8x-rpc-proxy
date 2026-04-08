# d8x-rpc-proxy

Transparent Ethereum JSON-RPC proxy. Receives standard JSON-RPC POST requests from frontends and forwards them to private RPC endpoints, cycling through available providers via [globalrpc](https://github.com/D8-X/globalrpc). The private RPC URLs are never exposed to the client.

## Requirements

- Redis instance (used by globalrpc for endpoint locking)

## Configuration

### RPC config file

Create a JSON file listing your RPC endpoints per chain (see `config/rpc-config.example.json`):

```json
[
  {
    "chainId": 42161,
    "https": [
      "https://your-private-rpc-1.example.com",
      "https://your-private-rpc-2.example.com"
    ],
    "wss": []
  }
]
```

### Environment variables

Copy `.env.example` to `.env` and fill in your values:

| Variable | Default | Description |
|---|---|---|
| `CHAIN_ID` | — | Chain ID to proxy (required) |
| `RPC_CONFIG_FILE` | `rpc-config.json` | Path to the RPC config JSON file |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | — | Redis password (optional) |
| `LISTEN_ADDR` | `:8080` | Address and port to listen on |

## Running

### Locally

```bash
cp config/rpc-config.example.json config/rpc-config.json
# edit config/rpc-config.json with your endpoints

cp .env.example .env
# edit .env

export $(cat .env | xargs)
go run ./cmd
```

### Docker

Build from the repo root:

```bash
docker build -f cmd/Dockerfile -t d8x-rpc-proxy .
```

Run:

```bash
docker run --env-file .env d8x-rpc-proxy
```

## Usage

The proxy exposes a single endpoint at `POST /rpc`. Send standard Ethereum JSON-RPC requests to it:

```bash
curl -X POST https://your-proxy-host/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

A health check is available at `GET /health`.
