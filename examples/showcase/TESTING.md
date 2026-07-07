# Showcase — manual test guide

Run the stack, then exercise routes with the curls below.

```bash
make dev-up
go run ./examples/showcase
```

Base URL: `http://127.0.0.1:8080`

Optional: set a client correlation id (echoed as `client_request_id` in responses):

```bash
export H='X-Client-Request-Id: manual-test-1'
```

---

## Ops

```bash
# Health (Postgres ping via repo facade)
curl -s $H http://127.0.0.1:8080/health | jq .

# Version / build metadata
curl -s $H http://127.0.0.1:8080/version | jq .
```

---

## OpenAPI documents

```bash
curl -s $H http://127.0.0.1:8080/openapi.json | jq '.info.title, (.paths | keys)'
curl -s $H http://127.0.0.1:8080/demo/openapi.json | jq '.info.title, (.webhooks | keys), (.components.schemas | keys)'
curl -s $H http://127.0.0.1:8080/mobile/openapi.json | jq '.info.title, (.paths | keys)'
curl -s $H http://127.0.0.1:8080/console/openapi.json | jq '.info.title, (.paths | keys)'
curl -s $H http://127.0.0.1:8080/mobile/v1/users/manage/openapi.json | jq '.info.title, (.paths | keys)'
```

---

## Core API (`/api/v1`) — Postgres users + cross-DB audit

```bash
# List users (page pagination in envelope.pagination)
curl -s $H 'http://127.0.0.1:8080/api/v1/users?page=1&size=10' | jq .

# Fetch one user
curl -s $H http://127.0.0.1:8080/api/v1/users/1 | jq .

# User not found → 404 catalog 10002
curl -s $H -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/api/v1/users/999

# Update user (version must be 1 or → 409 stale)
curl -s $H -X PUT http://127.0.0.1:8080/api/v1/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Updated User","email":"updated@example.com","version":1}' | jq .

# Stale version → 409
curl -s $H -o /dev/null -w '%{http_code}\n' -X PUT http://127.0.0.1:8080/api/v1/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"X","email":"x@example.com","version":99}'

# Duplicate email → 409 (use seeded email on another id if present)
# curl -s $H -X PUT ... -d '{"name":"X","email":"user@example.com","version":1}'

# Audit trail (defaults limit=20; core user check + demo DB read)
curl -s $H http://127.0.0.1:8080/api/v1/users/1/audit | jq .
curl -s $H 'http://127.0.0.1:8080/api/v1/users/1/audit?limit=5' | jq .
```

---

## Demo items (`/demo`) — memory store

```bash
# Cursor list (default limit=10)
curl -s $H http://127.0.0.1:8080/demo/items | jq .
curl -s $H 'http://127.0.0.1:8080/demo/items?limit=1' | jq '.data.items, .cursor'

# Fetch item
curl -s $H http://127.0.0.1:8080/demo/items/1 | jq .

# Not found → 404
curl -s $H -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/demo/items/3

# Corrupt row demo → 500 (internal mapped)
curl -s $H http://127.0.0.1:8080/demo/items/99 | jq .

# Create item
curl -s $H -X POST http://127.0.0.1:8080/demo/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"widget","email":"widget@example.com"}' | jq .

# Validation collection → 422 (multiple errors)
curl -s $H -X POST http://127.0.0.1:8080/demo/items \
  -H 'Content-Type: application/json' -d '{}' | jq .
curl -s $H -X POST http://127.0.0.1:8080/demo/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"","email":"bad"}' | jq .

# Patch name
curl -s $H -X PATCH http://127.0.0.1:8080/demo/items/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"alpha-renamed"}' | jq .

# Delete item
curl -s $H -X DELETE http://127.0.0.1:8080/demo/items/2 | jq .
curl -s $H -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/demo/items/2
```

---

## HTTP QUERY, headers, cookies, raw JSON

```bash
# HTTP QUERY method (OpenAPI "query" operation)
curl -s $H --request QUERY http://127.0.0.1:8080/demo/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"alp"}' | jq .

# Header + cookie binding
curl -s $H http://127.0.0.1:8080/demo/preferences \
  -H 'Accept-Language: de' -b 'theme=dark' | jq .

# Non-enveloped JSON (imperative Fiber route)
curl -s $H http://127.0.0.1:8080/demo/raw

# Binary export (imperative; documented in OpenAPI via Describe)
curl -s $H -o /tmp/item-export.txt -w '%{http_code}\n' http://127.0.0.1:8080/demo/export/1
cat /tmp/item-export.txt
```

---

## Multipart upload

```bash
curl -s $H -F title=report -F file=@examples/README.md \
  http://127.0.0.1:8080/demo/upload | jq .
```

---

## Auth & security

```bash
# Login → save token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/demo/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"secret"}' | jq -r '.data.token')
echo "token=$TOKEN"

# Empty body → 422 validation
curl -s $H -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8080/demo/auth/login \
  -H 'Content-Type: application/json' -d '{}'

# Bad password → 401
curl -s $H -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8080/demo/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"wrong"}'

# Bearer
curl -s $H http://127.0.0.1:8080/demo/me -H "Authorization: Bearer $TOKEN" | jq .

# API key header
curl -s $H http://127.0.0.1:8080/demo/me -H "X-API-Token: $TOKEN" | jq .

# Session cookie scheme
curl -s $H http://127.0.0.1:8080/demo/session -b "demo_session=$TOKEN" | jq .

# No auth → 401
curl -s $H -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/demo/me

# Admin route (header token + username check)
curl -s $H http://127.0.0.1:8080/demo/admin -H "X-API-Token: $TOKEN" | jq .

# OAuth scope demo:demo:admin → 403 for demo user
curl -s $H -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/demo/scoped-admin \
  -H "Authorization: Bearer $TOKEN"
```

---

## Multi-surface stubs (`/mobile`, `/console`)

```bash
curl -s $H http://127.0.0.1:8080/mobile/v1/users/1 | jq .
curl -s $H -X POST http://127.0.0.1:8080/mobile/v1/users/manage \
  -H 'Content-Type: application/json' \
  -d '{"action":"activate","note":"ok"}' | jq .
curl -s $H http://127.0.0.1:8080/mobile/ping | jq .
curl -s $H http://127.0.0.1:8080/console/v1/users | jq .
curl -s $H http://127.0.0.1:8080/console/ping | jq .
```

---

## Escape hatches (imperative Fiber)

Internal details must not appear on the wire.

```bash
# Internal catalog error → generic 500
curl -s $H http://127.0.0.1:8080/demo/leak | jq .

# Plain error → generic 500
curl -s $H http://127.0.0.1:8080/demo/plain | jq .
```

---

## Docs-only routes (OpenAPI spec, no live handler)

These appear in `/demo/openapi.json` but are not mounted as Fiber handlers:

- `GET /demo/ws` — WebSocket 101
- `POST /demo/stream` — SSE `TokenChunk` item schema
- `GET /demo/export/:id` — documented alongside the imperative export handler

---

## Quick smoke script

```bash
BASE=http://127.0.0.1:8080
for path in /health /openapi.json /demo/openapi.json /mobile/openapi.json \
  /api/v1/users /api/v1/users/1/audit /demo/items; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE$path")
  echo "$path -> $code"
done
```

Expected: all `200`.
