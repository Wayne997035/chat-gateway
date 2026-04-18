# chat-gateway

Real-time encrypted chat rooms via dual-protocol design.

## Architecture

- **HTTP Gateway** (port 8080) — Gin REST API + SSE streaming
- **gRPC Service** (port 8081) — Core business logic
- **MongoDB** — Rooms, messages, encrypted keys

```
Client → HTTP (Gin) → gRPC service → MongoDB
                            ↓
                      SSE broadcast → subscribers
```

## Key Design Decisions

- **Encryption**: `MASTER_KEY` env var → encrypts per-room AES-256-GCM keys in MongoDB. Missing = random key (dev only)
- **Key manager**: Double-Check Locking with `sync.RWMutex`
- **Pagination**: cursor-based (not offset)
- **Rate limiting**: 3-tier (global / endpoint / IP), relaxed in `configs/local.yaml`
- **Config**: `APP_ENV` selects `configs/{local,staging,development}.yaml`

## Commands

```bash
task check        # MUST run before every commit (lint + test)
task build        # Docker build + push
task deploy ENV=stg
```
