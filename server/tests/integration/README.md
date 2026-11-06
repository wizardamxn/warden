# Integration tests

Tests that run against a real Postgres, because claiming and saving depend on what Postgres does. They never call the real Gemini API or YouTube.

- From `server/`: `go test -count=1 ./tests/integration`
- They use the local Postgres from `deploy/compose.yaml`, or `TEST_DATABASE_URL` if set. With neither reachable they skip, and the skip message says why. Run with `-v` to see it: a plain `ok` can mean every test skipped.
- `DATABASE_URL` is never used, so tests can't reach a real database. Each test makes its own schema and drops it.
