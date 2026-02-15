# GopherKV

A lightweight key-value storage service in Go with HTTP API, CLI, TTL, memory limit, and AOF+RDB persistence.

## Run

```bash
go run ./cmd/kvd -config configs/config.yaml
```

## CLI

```bash
go run ./cmd/kvcli -h 127.0.0.1 -p 6380
```

Commands:

- `set <key> <value> [ttl <seconds>]`
- `get <key>`
- `del <key>`
- `exists <key>`
- `ttl <key>`
- `stats`
- `snapshot`
- `help`
- `exit | quit`

## HTTP API

- `PUT /v1/key`
- `GET /v1/key?k=<key>`
- `DELETE /v1/key?k=<key>`
- `GET /v1/exists?k=<key>`
- `GET /v1/ttl?k=<key>`
- `GET /v1/stats`
- `POST /v1/snapshot`
- `GET /v1/health`

Response format:

```json
{"code":0,"data":{},"msg":"ok"}
```

## Test

```bash
GOCACHE=.gocache go test ./...
```
