git clone <repo-url>
# SelfStack 🔐

**Pay-per-use file hosting with on-prem friendly deployment**

Store files and pay only for what you use—per GB, per hour. Deploy on your own infra, keep data local, and use Open Payments / Web Monetization for granular billing. Internet access is only required for payment settlement, not for data transfer.

![SelfStack](https://img.shields.io/badge/ILP-Hackathon%202025-emerald)

## Highlights

- 📁 Upload any file type via web UI (drag-and-drop or browse)
- 🔒 Data can stay entirely on-prem; storage and access do not require external internet
- 💰 Pay-per-use economics; balance freezes uploads/downloads when negative
- 🔗 Expiring share links (Phase 5) with audited charges and HTTPS downloads
- ⚡ Open Payments / Interledger integration for top-ups; minimum non-zero charge enforced
- 🛰️ Resilient to slow or intermittent outbound connectivity
- 🗄️ Open-source S3-compatible storage backends supported (Hetzner, MinIO, etc.)
- 📦 Upload queue with visible state and cancel of current item
- 🕒 Daily storage charges (billed per hour; see pricing)
- ❄️ Account freeze when not active/insufficient balance (blocks uploads and downloads)
- 🛠️ Admin CLI available for account management

## Features

- Pay per GB per hour; simple pricing (see below)
- Real-time balance polling and fixed-point ledger (4 decimal places)
- Proxy uploads to S3-compatible storage; presigned downloads
- Secure, expiring share links (default 7 days); owner charged only on successful download
- Activity tracking and policy engine to block when accounts are frozen
- Web client shows upload queue, cancel current upload, and copyable share links
- Interledger Web Monetization activates after login and tracks paid credits via the credit scheme

## Tech Stack

- **Frontend:** Next.js 16 (App Router), React, Tailwind CSS, Lucide icons
- **Backend:** Go (Echo), fixed-point ledger, policy engine
- **Storage:** S3-compatible object storage (Hetzner, AWS S3, etc.) with presigned URLs
- **Payments:** Open Payments / Interledger Web Monetization
- **Build tools:** pnpm for frontend, Go 1.24+ for server

## Architecture (Phase 5)

```
Web Client (uploads via backend proxy, balance + sharing UI)
				│
				▼ HTTP
Echo Web Server (auth, policy, S3 client)
				│
				▼
Core Logic: identity (SHA256), ledger (fixed-point), policy, activity, monetization
				│
				▼
Object Storage (S3-compatible)
s3://<bucket>/users/<user-hash>/<upload-id>-<filename>
```

## Pricing & Credits

- Example pricing: $0.001/GB/hour (~$0.72/GB/month)
- Credits use scale=4 (1 credit = 10,000 raw units)
- Minimum non-zero charge of 1 raw unit for non-zero-sized transfers
- Ingress/egress minimum charge of 0.0001 credits prevents abuse via huge numbers of tiny files
- Account states: `Active` (balance ≥ 0), `Frozen` (balance < 0; uploads/share downloads blocked)

| Storage | Cost |
|---------|------|
| 1 GB for 1 hour | $0.001 |
| 1 GB for 1 day | $0.024 |
| 1 GB for 1 month | ~$0.72 |

## Sharing (Phase 5+)

- One-click share link per file; links are time-limited (default 7 days)
- Redeem via `GET /share/:token` (no login); owner charged egress only after successful download
- Operator policy enforced: frozen accounts block downloads; events are logged with `[CHARGE]`

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 18+ and pnpm
- S3-compatible storage (endpoint, region, bucket, keys)
- Modern browser

### Configuration (.localconfig.yaml for dev)

```yaml
server:
	port: 8080
	redis:
		enabled: false
auth:
	enabled: false           # dev mode; enable and configure for prod
credits:
	scale: 4
	units_per_credit: 10
	ingress_per_gib: 15000
	egress_per_gib: 30000
	storage_per_gib_month: 10000
s3:
	endpoint: https://hel1.your-objectstorage.com
	region: hel1
	bucket: your-bucket-name
	access_key: your-access-key
	secret_key: your-secret-key
	presigned_url_expiry_upload: 3600
	presigned_url_expiry_download: 300
monetization:
	enabled: false
```

### Build & Run the Server

```bash
cd server
go build -o selfstack .

# Dev mode (X-User-ID auth, auto .localconfig.yaml)
./selfstack -dev

# Dev with explicit config
./selfstack -dev -config my-config.yaml

# Production
./selfstack -config /etc/selfstack/config.yaml
```

### Run the Frontend

```bash
pnpm install
pnpm dev      # http://localhost:3000 (Next.js)
```

## API (server)

**Auth (dev):** `POST /login` (X-User-ID header)

**Account:** `GET /user`, `GET /balance`, `POST /credit`, `POST /debit`, `GET /policy`, `GET /dev/boost`

**Files (S3 via proxy):**
- `POST /files/upload-url` – allocate uploadId, charge credits, echo filename
- `POST /files/:uploadId/upload` – proxy upload; stores `uploadId-filename`
- `POST /files/:uploadId/complete` – mark upload complete
- `GET /files/:uploadID/download` – presigned GET
- `DELETE /files/:uploadID` – delete from S3
- `GET /files` – list files for the user
- `POST /files/:uploadId/share` – create share link
- `GET /share/:token` – redeem share link

**Health & Config:** `GET /health`, `GET /monetization/config` (returns `units_per_credit`)

## Usage Flow

1) **Add credits (dev):** `curl http://localhost:8080/dev/boost -H "X-User-ID: alice@example.com"`
2) **Login:** open the web UI, enter email (dev) or use Google OAuth (prod)
3) **Upload:** drag/drop or select; uploads auto-start, queue is visible, cancel current item
4) **Share:** click Share, copy expiring link; owner is charged only on successful download
5) **Download:** uses presigned URLs resolved by uploadId
6) **Monitor balance:** web UI polls `/user` every 10s; `GET /balance` available

## Production Notes

- Enable Google OAuth (`auth.enabled: true`) and provide config
- Store S3 credentials securely; prefer IAM roles where possible
- Rate-limit presigned URL generation; enable HTTPS everywhere
- Increase logging/auditing for share and charge events

## Security Notes

## Admin CLI (examples)

CLI requires Redis ledger (no in-memory). Flags must come **before** commands when invoking the binary.

```bash
# List users with balances, file counts, and total storage size
./selfstack -config /etc/selfstack/config.yaml list

# Boost credits for a user (amount in raw units; scale=4 → 10000 = 1.0000 credit)
./selfstack -config /etc/selfstack/config.yaml boost alice@example.com 50000

# Drain a user to zero balance
./selfstack -config /etc/selfstack/config.yaml drain alice@example.com

# Delete a user (removes files and zeroes balance)
./selfstack -config /etc/selfstack/config.yaml delete alice@example.com

# Danger: delete every user and all data
./selfstack -config /etc/selfstack/config.yaml dropall
```

## Security Notes
- Dev mode (`-dev`, `auth.enabled:false`) is **not secure**; use only locally
- Presigned URLs expire (upload default 1h, download default 5m); request new ones if expired
- Minimum charge prevents zero-cost abuse for tiny files

## Nice to haves (future)

- End-to-end encryption and session keys for sharing
- Folder support in the UI and API

## Known issues

- All traffic currently proxied through the server because Hetzner S3 lacks CSP support. In self-hosted S3 with CSP, direct paths can be re-enabled to speed up uploads/downloads.

## Troubleshooting

- "Missing Authorization" – ensure `-dev` and `X-User-ID` in dev, or OAuth token in prod
- S3 errors – verify endpoint/credentials/bucket; test with `aws s3 ls --endpoint-url ...`
- "Insufficient balance" – credit is charged upfront on upload allocation
- URL expired – regenerate presigned upload/download URL

## License

MIT

## Built for ILP Hackathon 2025

Demonstrates micropayments for file storage using Open Payments / Interledger.

---

Made with 💚 using Open Payments
