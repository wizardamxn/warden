# Testing

How we test the website, the API and the worker.

## Commands

- Server, from `server/`: `go test ./...`
- Website, from `apps/web/`: `npm run lint`, `npm run typecheck`, `npm run test:e2e`, `npm run build`. Stop `npm run dev` before `npm run build`.
- No test calls the real Gemini API.
- Server tests need the local Postgres running for the database tests. Use `go test -count=1 -v ./...`: without `-v`, a plain `ok` can mean every database test skipped.
- Website tests mock the API, so they don't need Go running.
- Website mocks live in `apps/web/e2e/mock.ts`. The mocked `/v1/me` and login-only routes need the `warden_session` cookie, and any unmocked call to 127.0.0.1:8080 is refused, so no test can reach a real API.
