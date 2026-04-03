# Operator Setup Guide

Complete guide for running an Obsideo storage provider: build, configure, register, verify, and operate.

> **Alpha software.** See [Limitations](#limitations) before running in production.

---

## Contents

1. [Prerequisites](#prerequisites)
2. [Build](#build)
3. [Configuration](#configuration)
4. [Coordinator public key](#coordinator-public-key)
5. [Starting the provider](#starting-the-provider)
6. [Running as a service](#running-as-a-service)
7. [Registering with a coordinator](#registering-with-a-coordinator)
8. [Verifying the full lifecycle](#verifying-the-full-lifecycle)
9. [Production checklist](#production-checklist)
10. [Troubleshooting](#troubleshooting)
11. [Limitations](#limitations)

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Go 1.22 or later | Only needed to build. The binary has no runtime dependencies. |
| Reachable address | The coordinator sends HTTP requests to your provider at registration and every challenge cycle (every 8 hours). Your provider's address must be reachable by the coordinator — a public URL, a tunnel, or a shared private network. |
| Coordinator public key | A PEM file provided by the coordinator operator. The provider refuses to start without it. |
| Disk space | Size this to the `capacity_bytes` you plan to advertise. The provider does not enforce limits internally. |

---

## Build

```bash
git clone https://github.com/Regan-Milne/obsideo-storage-provider.git
cd obsideo-storage-provider
go build -o provider .
```

Produces a single self-contained binary named `provider`. No libraries are installed separately.

To verify:

```bash
./provider --help
```

---

## Configuration

```bash
cp config.example.yaml config.yaml
```

`config.yaml` is gitignored and never committed. Edit it:

```yaml
provider_id: "my-provider-1"   # Human-readable label. Pick anything. Not used for auth.

server:
  host: "0.0.0.0"              # Bind to all interfaces. Use "127.0.0.1" for localhost-only.
  port: 3334

data:
  path: "./data"               # Root directory for stored objects. Use an absolute path
                               # in production (e.g. /var/lib/obsideo-provider).

tokens:
  public_key_path: "coordinator_pub.pem"
```

All fields have defaults. The only field you must set before starting is `tokens.public_key_path` — or leave it as `coordinator_pub.pem` and place the file there.

---

## Coordinator public key

The provider verifies every upload and download token against the coordinator's Ed25519 public key. This file must be in place before starting the provider.

Obtain the file from the coordinator operator. It looks like:

```
-----BEGIN PUBLIC KEY-----
<32 bytes of Ed25519 public key, base64-encoded>
-----END PUBLIC KEY-----
```

Place it at the path specified in `config.yaml` (default: `coordinator_pub.pem` in the working directory):

```bash
# Example: copy from coordinator machine
scp coordinator-host:/path/to/coordinator/data/coordinator.pub ./coordinator_pub.pem
```

The provider will print an error and exit if the file is missing or malformed.

---

## Starting the provider

```bash
./provider start --config config.yaml
```

Expected output:

```
provider-clean listening on 0.0.0.0:3334
```

On first run the provider creates `data/objects/` and `data/index/` automatically.

**Health check:**

```bash
curl http://localhost:3334/health
# → {"status":"ok"}
```

The coordinator calls this endpoint at registration time to confirm your provider is reachable.

---

## Running as a service

### systemd (Linux)

Create `/etc/systemd/system/obsideo-provider.service`:

```ini
[Unit]
Description=Obsideo storage provider
After=network.target

[Service]
Type=simple
User=obsideo
WorkingDirectory=/opt/obsideo-provider
ExecStart=/opt/obsideo-provider/provider start --config config.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
# Create a dedicated user (recommended)
useradd -r -s /sbin/nologin obsideo

# Install files
install -d -o obsideo /opt/obsideo-provider
cp provider config.yaml coordinator_pub.pem /opt/obsideo-provider/
chown -R obsideo:obsideo /opt/obsideo-provider

# Enable and start
systemctl daemon-reload
systemctl enable obsideo-provider
systemctl start obsideo-provider

# Check status
systemctl status obsideo-provider
journalctl -u obsideo-provider -f
```

### macOS launchd

Create `~/Library/LaunchAgents/com.obsideo.provider.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.obsideo.provider</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/obsideo-provider</string>
    <string>start</string>
    <string>--config</string>
    <string>/usr/local/etc/obsideo-provider/config.yaml</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/var/log/obsideo-provider.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/obsideo-provider.log</string>
</dict>
</plist>
```

```bash
launchctl load ~/Library/LaunchAgents/com.obsideo.provider.plist
```

---

## Registering with a coordinator

Registration is a two-step process: register (coordinator performs a liveness check), then an operator approves you.

### Step 1 — Register

Your provider must already be running and reachable at the address you specify.

```bash
COORDINATOR="http://<coordinator-host>:8080"
YOUR_ADDRESS="https://<your-public-address>"   # or http://127.0.0.1:3334 for local dev
CAPACITY_BYTES=107374182400                     # 100 GiB; set to your actual available space

REG=$(curl -s -X POST "${COORDINATOR}/internal/providers/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"address\":        \"${YOUR_ADDRESS}\",
    \"connectivity\":   \"direct\",
    \"public_key\":     \"unused-v1\",
    \"capacity_bytes\": ${CAPACITY_BYTES}
  }")

echo "$REG"
# → {"id":"<uuid>","status":"pending","address":"...","score":1,...}

# Save your provider ID
PROVIDER_ID=$(echo "$REG" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "Your provider ID: $PROVIDER_ID"
```

The coordinator calls `GET {your-address}/health` before accepting the registration. If it cannot reach you, the registration will fail. See [troubleshooting](#troubleshooting) if this happens.

### Step 2 — Approval

A coordinator operator must approve your provider before it receives any uploads:

```bash
# Coordinator operator runs this on their side:
curl -s -X POST "${COORDINATOR}/internal/providers/${PROVIDER_ID}/approve"
# → {"id":"...","status":"active",...}
```

Contact the coordinator operator with your provider ID. Once approved, your provider's `status` becomes `active`.

### Check your status

```bash
curl -s "${COORDINATOR}/internal/providers/${PROVIDER_ID}"
```

Key fields:

| Field | Meaning |
|-------|---------|
| `status` | `pending` — awaiting approval. `active` — receiving uploads and being challenged. `suspended` — operator action required. |
| `score` | Float `[0.0, 1.0]`. Starts at 1.0. Decreases on challenge failure (−0.1), recovers on pass (+0.01). Providers below 0.7 are excluded from new uploads but continue to be challenged. |
| `used_bytes` | Bytes in use as tracked by the coordinator. |
| `last_heartbeat` | Last confirmed liveness timestamp. |

---

## Verifying the full lifecycle

After your provider is active, confirm the full storage path works.

### Manual challenge test

Upload any file, then manually challenge your provider to prove it holds the file:

```bash
# After uploading a file, get its merkle root (the hex string in the upload output)
MERKLE="<merkle-hex>"

curl -s -X POST http://localhost:3334/challenge \
  -H "Content-Type: application/json" \
  -d "{
    \"challenge_id\": \"manual-test-1\",
    \"merkle\":       \"${MERKLE}\",
    \"chunk_index\":  0,
    \"nonce\":        \"aabbccddeeff0011\",
    \"expires_at\":   9999999999
  }"
```

Expected response:

```json
{
  "challenge_id":      "manual-test-1",
  "chunk_hash":        "<64-character hex>",
  "total_chunk_count": 1
}
```

If you get a `404`, the object was not stored on this provider. If you get a `400`, check the request format — `expires_at` must be an integer (Unix timestamp), not a string.

### Full lifecycle with the upload tool

If you have access to the coordinator's upload tool:

```bash
./upload \
  --file     /path/to/any/file \
  --bucket   test-bucket \
  --key      test.txt \
  --api-key  <your-api-key> \
  --coordinator http://<coordinator>:8080 \
  --lifecycle-check
```

A passing run confirms: upload, download with byte-exact verification, coordinator-side delete, GC cycle, and physical deletion from the provider.

```
[upload      ] [1/1] → <your-provider-id> (<your-address>)
[confirm     ] OK
[download    ] PASS — N bytes match exactly
[delete      ] coordinator mapping removed (204)
[gc          ] provider <id> — merkle root gone from /list
[lifecycle   ] PASS — upload, download, delete, and GC all verified
```

---

## Production checklist

Before accepting real traffic:

- [ ] Provider address is HTTPS (TLS-terminating reverse proxy in front, or direct TLS)
- [ ] `data/` directory is on a persistent volume that survives reboots
- [ ] `data/` is backed up — losing it means losing all stored files and failing all future challenges for those objects
- [ ] Provider is running as a non-root user with write access only to `data/`
- [ ] Internal endpoints are firewalled to the coordinator's source IP
- [ ] `coordinator_pub.pem` is the live coordinator's current public key
- [ ] Service restarts automatically on crash (`Restart=on-failure`)
- [ ] Disk space monitoring is in place — the provider does not enforce capacity limits internally
- [ ] `provider_id` in config is descriptive and unique across your deployments

### Firewall rules

In production, restrict these unauthenticated endpoints to the coordinator's IP:

```
POST /challenge
POST /replicate
DELETE /objects/*
GET  /list
```

The authenticated endpoints (`/upload/*`, `/download/*`) and `/health` must be reachable from the coordinator and from clients.

---

## Troubleshooting

### Provider fails to start: "load coordinator public key"

The file at `tokens.public_key_path` is missing, empty, or malformed.

```bash
# Confirm the file exists
ls -la coordinator_pub.pem

# Confirm it has a valid PEM header
head -1 coordinator_pub.pem
# Should print: -----BEGIN PUBLIC KEY-----

# Confirm key length (Ed25519 = 32 bytes = 256 bits)
openssl pkey -pubin -in coordinator_pub.pem -text -noout 2>/dev/null | grep bit
# Should print: 256 bit (Ed25519)
```

If the file is missing: obtain it from the coordinator operator. The coordinator writes it to `data/coordinator.pub` on first run.

---

### Registration fails: "liveness check failed" or connection refused

The coordinator tried `GET {your-address}/health` and could not reach it.

1. Confirm your provider is running: `curl http://localhost:3334/health`
2. Confirm the address in your registration request is reachable **from the coordinator's machine**, not just from your own
3. If using a tunnel (ngrok, Cloudflare Tunnel, etc.): confirm the tunnel is active and the URL matches your registration body exactly
4. Check firewall rules — inbound connections on port 3334 (or your configured port) must be allowed

---

### Upload tokens rejected (401 Unauthorized)

The token signature does not verify against `coordinator_pub.pem`.

- The most common cause: `coordinator_pub.pem` is from a different coordinator instance, or the coordinator was restarted and generated a new key pair
- Copy the latest `coordinator.pub` from the coordinator and restart the provider

---

### Challenge returns 400

The challenge request body could not be parsed.

- Confirm `expires_at` is an integer in your request, not a quoted string. The coordinator sends it as a Unix timestamp (`int64`). Some test scripts mistakenly quote it.
- If the coordinator is sending a `400`: check the coordinator logs for the raw request body

---

### Challenge returns 404

The provider does not have the object.

- Confirm the upload was directed to this provider (check the coordinator's object record — it lists which provider IDs hold each file)
- If the `data/` directory was wiped or moved, the object is gone — the coordinator will replicate it to a different provider and your score will drop

---

### Score is dropping

The coordinator is issuing challenges that your provider is failing.

1. Check coordinator logs for `challenge error` lines — they include the merkle root and error
2. Run the manual challenge test (above) to confirm your provider responds correctly to a known-good request
3. If your `data/` directory was reset, you have lost stored files — contact the coordinator operator

Score recovery: +0.01 per passing challenge cycle (every 8 hours). To recover from 0.6 to above the 0.7 routing floor takes approximately 10 cycles (~3.3 days). There is no manual score reset in v1.

---

### `data/objects/` has files without matching entries in `data/index/`

This can happen if the provider crashed between writing the object file and writing the index file. Affected objects cannot pass challenges.

Safe remediation: delete the orphaned object files (those without a matching `.json` in `data/index/`). The coordinator will notice the next challenge failure and trigger replication from another provider.

---

### Provider runs out of disk space

The provider does not refuse uploads when full. If the disk fills up, writes will fail with OS-level errors and uploads will return 500.

Monitor disk usage externally. The `capacity_bytes` you registered is tracked by the coordinator but not enforced by the provider. Set `capacity_bytes` conservatively to avoid being assigned more data than you can store.

---

## Limitations

| Area | Current state |
|------|---------------|
| **Auth on internal endpoints** | `/challenge`, `/replicate`, `/list`, `DELETE /objects/{merkle}` are unauthenticated. Restrict to coordinator IP at the firewall. |
| **No built-in TLS** | Provider serves plain HTTP. Use nginx, Caddy, or a load balancer for HTTPS. |
| **Single-node** | No clustering. One process per data directory. |
| **No capacity enforcement** | Provider does not reject uploads when `capacity_bytes` is exceeded. Monitor disk externally. |
| **Manual approval** | All new providers require coordinator operator action to activate. |
| **Slow score recovery** | +0.01 per 8-hour challenge cycle. Full recovery from 0.0 to 1.0 takes ~33 days. |
| **No address update** | If your public address changes, re-register with the new address. |
| **Replication at scale** | Replication flow works in testing but has not been validated under large files or high concurrency. |
| **Alpha network** | The coordinator network is pre-production. Do not use this for user data that matters. |
