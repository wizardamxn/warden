# API program

Starts the API. Runs on the phone and is reached through the Cloudflare Tunnel, once login exists. Until then it stays on 127.0.0.1. See [internal/api](../../internal/api/ARCHITECTURE.md).

From `server/`: `go run ./cmd/api`
