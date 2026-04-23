# Repository archived: use `obsideo-provider` instead

**This repository has been archived.** The canonical Obsideo storage provider repo is now:

### [github.com/Regan-Milne/obsideo-provider](https://github.com/Regan-Milne/obsideo-provider)

All active development, releases, and operator documentation live there. This repo is preserved read-only for historical reference.

**If you are an operator:** follow the release notes on the canonical repo. The current active release is [v2-2026-04-23-retention-auth](https://github.com/Regan-Milne/obsideo-provider/releases).

**If you linked here from somewhere else:** please update your link to point at `obsideo-provider`. This archive exists to preserve git history for anyone who followed a stale reference.

---

## Historical README (preserved below)

# Obsideo Storage Provider

> **Alpha / experimental.** This software is under active development. See [Limitations](#limitations) before running in production.

A storage provider node for the Obsideo decentralized storage network. Receives encrypted file uploads directly from clients, responds to periodic proof-of-storage challenges from the coordinator, and participates in automatic replication when other providers fail.

Single binary. No external runtime dependencies.

---

## How it works

```
Client SDK  ──upload token──►  Coordinator  ──challenge every 8h──►  Provider (this)
                ◄──download token──         ◄──replication trigger──
Client SDK  ──── raw bytes ────────────────────────────────────────►  Provider (this)
```

1. The coordinator issues a short-lived upload token scoped to your provider
2. The client uploads raw (encrypted) bytes directly to your provider
3. Every 8 hours the coordinator challenges each provider to prove it still holds each file
4. If a provider fails a challenge the coordinator triggers replication to a healthy provider

Your provider never sees plaintext data. Encryption and decryption happen client-side.

---

## Quickstart

**Requirements:** Go 1.22+, access to a running coordinator (address + public key).

```bash
# 1. Clone and build
git clone https://github.com/Regan-Milne/obsideo-storage-provider.git
cd obsideo-storage-provider
go build -o provider .

# 2. Get the coordinator's public key from the coordinator operator
#    and place it here:
cp /path/to/coordinator.pub ./coordinator_pub.pem

# 3. Configure
cp config.example.yaml config.yaml
# Edit config.yaml if needed — defaults work for local dev

# 4. Run
./provider start --config config.yaml
# → provider-clean listening on 0.0.0.0:3334

# 5. Health check
curl http://localhost:3334/health
# → {"status":"ok"}
```

That's it. Your provider is running. Next: [register it with the coordinator](OPERATOR_SETUP.md#registering-with-a-coordinator).

---

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness check. Always returns `{"status":"ok"}`. |
| `POST` | `/upload/{merkle}` | Store a file. Requires a coordinator-issued upload token. |
| `GET` | `/download/{merkle}` | Retrieve a file. Requires a coordinator-issued download token. |
| `POST` | `/challenge` | Respond to a proof-of-storage challenge from the coordinator. |
| `POST` | `/replicate` | Pull a file from a source provider and push it to a target. |
| `DELETE` | `/objects/{merkle}` | GC-triggered physical delete. |
| `GET` | `/list` | List all stored merkle roots. Used by coordinator GC. |

`/upload` and `/download` require `Authorization: Bearer <token>` where the token is a short-lived Ed25519 JWT issued by the coordinator. All other endpoints are unauthenticated — restrict them to the coordinator's source IP at your firewall in production.

---

## Data layout

```
data/
  objects/{merkle_hex}        raw file bytes (atomic write)
  index/{merkle_hex}.json     chunk metadata used to answer challenges
```

Both directories are created automatically on first run. Back them up together — they are not independently recoverable.

---

## Full setup guide

See **[OPERATOR_SETUP.md](OPERATOR_SETUP.md)** for:

- Detailed configuration reference
- Coordinator public key setup
- Running as a systemd service
- Registration and approval flow
- Lifecycle verification
- Production checklist
- Troubleshooting guide

---

## Limitations

This is alpha software on a pre-production network.

- Internal endpoints (`/challenge`, `/replicate`, `/list`, `DELETE /objects`) are unauthenticated — use firewall rules
- No built-in TLS; use a reverse proxy (nginx, Caddy) for HTTPS
- Single-node only; no clustering or shared storage
- Capacity limits are not enforced server-side
- Score recovery after failures is slow (+0.01 per 8h challenge cycle)
- All new providers require manual approval by a coordinator operator
- Replication has not been stress-tested at scale

See [OPERATOR_SETUP.md#limitations](OPERATOR_SETUP.md#limitations) for the full list.

---

## Contributing

This repository is part of the Obsideo platform. For bugs, questions, or operator support open an issue.
