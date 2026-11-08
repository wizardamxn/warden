# API program

Starts the API. Runs on the phone and listens on 127.0.0.1, where the Cloudflare Tunnel will reach it once the rest of [Before going public](../../internal/api/ARCHITECTURE.md#before-going-public) is done. See [internal/api](../../internal/api/ARCHITECTURE.md).

From `server/`: `go run ./cmd/api`
