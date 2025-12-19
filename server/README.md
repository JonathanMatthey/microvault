
# MicroVault

**Deployable web storage with transparent, pay-as-you-go economics**

Microvault is a web-based storage and data-sharing platform designed for environments where cost control, data locality, and network constraints matter. Deploy it on private infrastructure or restricted networks, while using Interledger Web Monetization for granular, usage-based payments.

---

## 🚀 What Microvault Is (and Is Not)

- **Requires internet connectivity only for Web Monetization payments.**
- **Data never needs to leave the local network.**
- **Storage and access can remain entirely on-premise.**
- **Internet access is required only for payment settlement, not data transfer.**

This makes Microvault ideal for:
- Slow or intermittent external connectivity
- Strict data-residency rules
- Controlled egress policies

---

## 🔗 File Sharing (Phase 5+)

Microvault supports secure, auditable file sharing via expiring links:

- **Share any uploaded file** with a one-click action in the web UI.
- **Links are time-limited** (default: 7 days) and can be redeemed by anyone with the URL.
- **No login required** to download via a share link, but the owner's balance is charged for egress only after a successful, complete download.
- **All share links are HTTPS** (even behind proxies).
- **Operator policy enforced:** If the owner's account is frozen, downloads are blocked.
- **Share events and charges are logged** with explicit `[CHARGE]` entries in the backend logs.
- **Frontend:** Share links are shown in a modal with copy-to-clipboard and expiry info.

**API Endpoints:**
- `POST /files/:uploadId/share` — Create a share link for a file you own.
- `GET /share/:token` — Redeem a share link (streams the file, charges owner on success).

---

## Phase 5 Architecture

MicroVault Phase 5 uses AWS S3 (or compatible object storage) with proxy uploads to S3 (no tus). Filenames are part of the S3 key (`uploadId-filename`).

```
┌─────────────────────────────────────────────────────────────┐
│                        Web Client                           │
│      (Uploads via backend proxy, balance + listing UI)      │
└─────────────────────────────────────────────────────────────┘
                            ↓ HTTP
┌─────────────────────────────────────────────────────────────┐
│                      Echo Web Server                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │  REST API    │  │  Middleware  │  │ S3 Client        │   │
│  │ (Auth, File) │  │  - Auth      │  │ (AWS SDK v2)     │   │
│  │              │  │  - Policy    │  │  - Proxy upload  │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Core Business Logic                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Identity   │  │    Ledger    │  │    Policy    │       │
│  │  (SHA256)    │  │ (fixed-point)│  │   Engine     │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Activity   │  │ S3 Storage   │  │  Monetization│       │
│  │   Tracker    │  │   Client     │  │   (opt)      │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Object Storage (S3)                      │
│      Hetzner Object Storage or AWS S3 Compatible            │
│   s3://<bucket>/users/<user-hash>/<upload-id>-<filename>    │
└─────────────────────────────────────────────────────────────┘
```


### Key Components

- **Identity**: User authentication via dev mode (X-User-ID header) or production Google OAuth with SHA256 hashing
- **Ledger**: In-memory credit accounting with fixed-point arithmetic (4 decimal places)
- **Policy Engine**: Enforces upload/download rules based on account balance
- **S3 Storage**: AWS S3 SDK v2 with presigned URLs for secure direct uploads/downloads
- **Activity Tracker**: Records user login activity for policy decisions
- **Web Client**: Auto-starts uploads on file selection, shows a single cancel control, manages a visible upload queue (current item highlighted), and supports file sharing via expiring links.



### Credit System

- **CreditScale**: 4 (1 credit = 10,000 internal units)
- **Ingress Cost**: File size determines cost (charged upfront when presigned URL is generated)
- **Egress Cost**: Downloads (including share link downloads) are charged per GiB, only after a successful, complete transfer.
- **Account States**:
  - `Active`: balance ≥ 0 (can upload/download/share)
  - `Frozen`: balance < 0 (uploads and share downloads blocked)


## Getting Started

### Prerequisites

- Go 1.24+
- S3-compatible object storage (Hetzner Object Storage, AWS S3, etc.)
- Node.js 18+ with pnpm (for frontend)
- A modern web browser

### Configuration

MicroVault uses a YAML configuration file (JSON still works for legacy setups). Create `.localconfig.yaml` for development:

```yaml
server:
  port: 8080
  redis:
    enabled: false
auth:
  enabled: false
credits:
  # Number of decimal places for credits shown to users
  scale: 4
  # Internal multiplier is derived as 10^scale (not configurable)
  # Effect: credits are stored as integers; displayed credits = raw / (10^scale)
  # Price per credit in minor currency units
  # Example: units_per_credit=10 means 10 minor currency units = 1.0 credit
  units_per_credit: 10
  ingress_per_gib: 15000          # credits to upload 1 GiB
  egress_per_gib: 30000           # credits to download 1 GiB
  storage_per_gib_month: 10000    # credits to store 1 GiB for a month
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

### Building the Server

```bash
# Build without any build tags (unified binary)
go build -o microvault .
```

The binary now supports both dev and production modes via CLI flags (see below).

### Running the Server

```bash
# Development mode (X-User-ID header auth, auto-detects .localconfig.yaml/.yml)
./microvault -dev

# Development with explicit config
./microvault -dev -config my-config.yaml

# Production mode (requires full config.yaml with auth settings)
./microvault -config /etc/microvault/config.yaml

# Show help
./microvault -help
```

The server will start on `http://localhost:8080` with the following endpoints:

### API Endpoints

#### Authentication (Dev Mode)
- `POST /login` - Login with X-User-ID header (dev mode only)

#### Account Management
- `GET /user` - Get user profile and credits
- `GET /balance` - Get account balance and state
- `POST /credit` - Add credits to account (dev mode)
- `POST /debit` - Deduct credits (dev mode)
- `GET /policy` - Check policy status (can upload/download)
- `GET /dev/boost` - Quick boost (+5 credits, dev mode only)


#### File Operations (Phase 5 - S3, proxy upload, and sharing)
- `POST /files/upload-url` - Allocate uploadId, charge credits, echo filename
- `POST /files/:uploadId/upload` - Proxy upload to S3 (body or multipart), stores to `uploadId-filename`
- `POST /files/:uploadId/complete` - Mark upload complete
- `GET /files/:uploadID/download` - Presigned GET; server finds the key by uploadId
- `DELETE /files/:uploadID` - Delete from S3; server finds the key by uploadId
- `GET /files` - List user's files (from S3)
- `POST /files/:uploadId/share` - Create a share link for a file you own
- `GET /share/:token` - Redeem a share link (streams the file, charges owner on success)


#### Health & Config
- `GET /health` - Health check endpoint
- `GET /monetization/config` - Monetization configuration
  - returns `units_per_credit` from the `credits` block

### Running the Frontend

```bash
cd sample-client

# Install dependencies (first time only)
pnpm install

# Start dev server
pnpm dev
```

The frontend will be available at `http://localhost:5173`.

## Usage Flow

### 1. Add Credits

You can add credits to your account in dev mode:

#### Quick Boost (Easiest for Testing)

```bash
# Add 5 credits (50,000 internal units) to alice@example.com
curl http://localhost:8080/dev/boost \
  -H "X-User-ID: alice@example.com"
```

#### Manual Credit Addition

```bash
# Add 10 credits (100,000 internal units) to alice@example.com
curl -X POST http://localhost:8080/credit \
  -H "X-User-ID: alice@example.com" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100000}'
```

### 2. Login (via Web UI)

1. Open `http://localhost:5173` in your browser
2. Enter your email (e.g., `alice@example.com`)
3. Verify server URL is `http://localhost:8080`
4. Click **Login**

The UI will display your current credit balance.

### 3. Upload Files (Phase 5 - Proxy to S3)

1. After login, drag & drop a file or click the upload area
2. Uploads **start automatically** on selection; additional selections are queued and shown in the **Upload Queue** with the active upload highlighted
3. The UI exposes a single **Cancel** button for the active upload; cancelling advances to the next queued item
4. Flow per file:
  - Request allocation via `POST /files/upload-url` (charges credits)
  - Upload file via backend proxy `POST /files/{uploadId}/upload` with `X-Filename`
  - Mark complete via `POST /files/{uploadId}/complete`
5. Credits are charged upfront during allocation
6. File appears in the list; download/delete/share use the same `uploadId`

### 6. Share Files

1. In the file list, click the **Share** button next to any file you own.
2. A modal will display a secure, expiring share link and its expiry time.
3. Anyone with the link can download the file (owner is charged for egress only after a successful download).
4. Share links expire automatically (default: 7 days).
5. All share events and charges are logged in the backend for auditability.


### 4. Monitor Balance

```bash
# Check balance via API
curl http://localhost:8080/balance \
  -H "X-User-ID: alice@example.com"
```

The web client polls `/user` every 10 seconds to keep the displayed credits fresh.

### 5. Download Files

```bash
# Get presigned download URL (server resolves the key by uploadId)
curl http://localhost:8080/files/<uploadId>/download \
  -H "X-User-ID: alice@example.com"

# Returns 302 redirect to presigned S3 URL
```

## Development

### Running Tests

```bash
# Run all tests (no build tags needed)
go test ./...

# Run specific package tests
go test ./internal/core/...

# Run with coverage
go test -cover ./...
```

Tests automatically enable dev mode via package-level initialization.

### Project Structure

```
microvault/
├── main.go                    # Unified entry point (dev + prod modes)
├── config.go                  # Configuration loading
├── auth.go                    # Google OAuth (production)
├── .localconfig.yaml          # Development configuration
├── deploy/
│   ├── config.yaml            # Production template config
│   └── ...                    # Deployment files
├── internal/
│   ├── core/                  # Constants & shared types
│   ├── identity/              # Authentication providers
│   │   ├── devlogin_provider.go
│   │   ├── oidc_provider.go
│   │   └── hash.go
│   ├── ledger/                # Credit accounting
│   │   ├── fixed/             # Fixed-point arithmetic
│   │   └── memory/            # In-memory ledger
│   ├── policy/                # Business rules engine
│   ├── activity/              # Activity tracking
│   ├── storage/
│   │   └── s3.go              # S3 client wrapper
│   └── upload/                # File upload handlers (Phase 5)
│       ├── upload_handler.go  # Upload allocation (costing)
│       ├── proxy_upload.go    # Backend proxy to S3 (uses filename in key)
│       ├── list_handler.go    # List files from S3
│       ├── complete_handler.go
│       ├── download.go        # Download via presigned URL (key lookup)
│       └── deletion.go        # Delete from S3 (key lookup)
├── sample-client/             # Web frontend
│   ├── index.html             # UI with login & upload
│   ├── main.js                # Presigned URL upload
│   └── package.json           # Frontend dependencies
└── design/                    # Design documents
    ├── 0.overview.md
    └── ...
```

## Security Notes

⚠️ **Development Mode**: This implementation uses `X-User-ID` header for authentication, which is **NOT SECURE** for production. It's designed for local development and testing only.

⚠️ **S3 Presigned URLs**: Development uses public S3 credentials in `.localconfig.yaml`. Never commit real credentials to version control.

### Production Recommendations

- Use Google OAuth (`-dev` flag not set, config requires `auth.enabled: true`)
- Store S3 credentials in environment variables or secure secret management
- Use IAM roles for EC2/container deployments
- Enable S3 bucket versioning and access logging
- Restrict bucket policies to specific IPs/roles
- Use S3 bucket encryption at rest
- Implement rate limiting on presigned URL generation
- Add request validation and sanitization
- Use HTTPS/TLS for all connections
- Add comprehensive audit logging

## Troubleshooting

### "Missing Authorization header" / "Invalid token"
- Ensure you're using `-dev` flag for development mode
- In dev mode, pass `X-User-ID` header instead of OAuth bearer token

### S3 Connection Errors
- Verify S3 credentials in `.localconfig.yaml`
- Check S3 endpoint URL is correct and accessible
- Ensure bucket name exists and credentials have access
- Test with AWS CLI: `aws s3 ls --endpoint-url <endpoint> --profile default`

### "Insufficient balance" error
- Charge costs the full file size upfront (not per-chunk)
- Add credits using `/dev/boost` or `/credit` endpoint

### Presigned URL expired
- Upload URLs expire after 1 hour (configurable)
- Download URLs expire after 5 minutes (configurable)
- If expired, request a new presigned URL

### Build errors with old tags
- Remove `-tags devlogin` from build commands
- Use `./microvault -dev` flag instead
- Old build tags no longer needed - everything is in one binary

## License

MIT
