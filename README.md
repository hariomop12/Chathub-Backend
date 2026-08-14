<div align="center">
  <h1>💬 ChatHub</h1>
  <p><strong>Real-time messaging platform with voice & video calling</strong></p>

  <p>
    <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go" alt="Go">
    <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react" alt="React">
    <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql" alt="PostgreSQL">
    <img src="https://img.shields.io/badge/GORM-ORM-00A86B" alt="GORM">
    <img src="https://img.shields.io/badge/WebSocket-Real--time-010101" alt="WebSocket">
    <img src="https://img.shields.io/badge/WebRTC-P2P-EE2C2C" alt="WebRTC">
    <img src="https://img.shields.io/badge/Cloudflare_R2-Storage-F38020?logo=cloudflare" alt="R2">
    <img src="https://img.shields.io/badge/Vite-Build-646CFF?logo=vite" alt="Vite">
    <img src="https://img.shields.io/badge/Dark%20Mode-🌙-000000" alt="Dark Mode">
  </p>

  <p>
    <a href="https://chat.hariomop.in"><strong>🌐 chat.hariomop.in</strong></a>
  </p>
</div>

---

## ✨ Features

- **Real-time messaging** — WebSocket-powered instant chat with typing indicators
- **Voice & video calls** — WebRTC-based peer-to-peer calling via PeerJS
- **File sharing** — Upload images, videos, and files to Cloudflare R2 (up to 50MB)
- **Direct & group chats** — 1-on-1 conversations and group messaging
- **Dark mode** — Light/dark theme toggle with localStorage persistence + system preference detection
- **Responsive design** — Mobile-first layout with adaptive sidebar, stacked views on small screens
- **User search** — Find users by name or email
- **Google Sign-In** — Auth with Google OAuth ID tokens (no password, no session store)
- **Auto user profile** — Users are created/updated automatically from their Google identity on each authenticated request
- **Modern UI** — Glass-morphism landing page, gradient accents, smooth transitions

## 🏗 Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   React     │────▶│   Go API    │────▶│  PostgreSQL │
│   Frontend  │     │   (Chi)     │     │   + GORM    │
│   Vite      │     │   :5000     │     │             │
└──────┬──────┘     └──────┬──────┘     └─────────────┘
       │                   │
       │  WebSocket        │  WebSocket
       │  (socket.io)      │  (gorilla/websocket)
       ▼                   ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  PeerJS     │     │  WebSocket  │     │ Cloudflare  │
│  Server     │     │    Hub      │     │     R2      │
│  :5001      │     │  (Go)       │     │  (S3 API)   │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Backend (Go)

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Router | `chi` v5 | Lightweight, idiomatic HTTP routing |
| ORM | `GORM` | PostgreSQL database access |
| Auth | `go-oidc` | Google Sign-In ID token verification (JWKS) |
| WebSocket | `gorilla/websocket` | Real-time bidirectional communication |
| Real-time fan-out | `Redis pub/sub` + transactional outbox | Cross-instance message delivery + at-least-once guarantee |
| File storage | `AWS SDK v2` (S3) | Cloudflare R2 file uploads |
| Migrations | `dbmate` | Database schema versioning |

### Frontend (React)

| Layer | Technology |
|-------|-----------|
| Framework | React 19 + TypeScript |
| Build | Vite 8 |
| Auth | Google Identity Services (`accounts.google.com/gsi/client`) |
| Routing | React Router v7 |
| Styling | CSS variables + inline styles |
| Theme | React Context + localStorage + `prefers-color-scheme` |
| WebSocket | Native WebSocket client |
| WebRTC | PeerJS |
| Icons | Lucide React |
| Image crop | react-easy-crop |

## 🚀 Getting Started

### Prerequisites

- Go 1.26+
- Node.js 24+
- PostgreSQL 16+
- PeerJS server (optional, for calls)

### Environment Variables

The app uses separate config for API, WebSocket, and PeerJS connectivity:

- `DATABASE_URL` - PostgreSQL connection string (used by the server and `dbmate` for migrations)
- `REDIS_URL` - optional Redis URL (e.g. Upstash `rediss://`). Enables cross-instance message fan-out via pub/sub; if unset the server degrades to single-instance in-process broadcast
- `GOOGLE_CLIENT_ID` - Google OAuth client id (Web application) used to verify Google Sign-In ID tokens
- `R2_*` / `S3_*` - Cloudflare R2 object storage credentials for uploads
- `CLIENT_URL` - frontend origin used by backend CORS
- `VITE_API_URL` - API base URL, usually `http://localhost:5000`
- `VITE_WS_URL` - optional explicit WebSocket URL, usually `ws://localhost:5000/ws`
- `VITE_PEER_HOST` - optional PeerJS host; if unset in production, calls are disabled gracefully
- `VITE_PEER_PORT` - PeerJS port, defaults to `5001`
- `VITE_PEER_PATH` - PeerJS path, defaults to `/peerjs`

### Backend

```bash
# Copy environment config
cp .env.example .env
# Edit .env with your own credentials

# Run database migrations (dbmate auto-loads .env)
dbmate up

# Start the server
go run ./cmd/server/
```

Or use the Makefile:

```bash
make run             # go run ./cmd/server
make build           # go build -o server ./cmd/server
make migrate         # dbmate up
make migrate-new     # dbmate new NAME=<name>
make migrate-status  # dbmate status
make migrate-down    # dbmate rollback
make vet             # go vet ./...
make test            # go test ./...
```

Server starts on **`http://localhost:5000`**

### Database Migrations

Migrations are managed with **dbmate** and live in `db/migrations/`.

**When do migrations run?**

- **Local dev:** run `make migrate` (or `dbmate up`) — dbmate reads `DATABASE_URL` from `.env`.
- **CI/CD (main push):** the `migrate` job in `.github/workflows/backend-image.yml` runs `dbmate --wait up` against `DATABASE_URL` **before** the Docker image is built and pushed. Add the `DATABASE_URL` secret in GitHub repo **Settings → Secrets and variables → Actions**.
- Migrations are **never** run inside the app container. They are applied explicitly before a new deploy rolls out, so the schema is updated first.

If your existing database already has tables created outside dbmate, `dbmate up` will fail with `relation already exists`. In that case either recreate the database, or baseline the existing schema before adding new migrations.

### Frontend

```bash
cd frontend

# Install dependencies
npm install

# Start dev server
npm run dev
```

App opens at **`http://localhost:5173`**

### PeerJS Server (for calls)

```bash
npx peerjs --port 5001 --path /peerjs
```

### Full Stack with Docker

```bash
docker compose up --build
```

- Frontend: `http://localhost:5174`
- Backend: `http://localhost:5002`
- PeerJS server: `http://localhost:5001/peerjs`

For production, set `VITE_PEER_HOST` to the actual PeerJS host or subdomain if you want calls. If it is not set, chat still works and the app skips call initialization safely.

### GitHub Container Registry

The backend image is published by GitHub Actions on pushes to `main`:

- Image: `ghcr.io/<your-github-username>/chathub-backend`
- Tags: `latest` and the short commit SHA
- Workflow: [`.github/workflows/backend-image.yml`](.github/workflows/backend-image.yml)

Pull it with:

```bash
docker pull ghcr.io/<your-github-username>/chathub-backend:latest
```

### API Documentation

Build a self-contained HTML doc and open it in the browser:

```bash
npx @redocly/cli build-docs openapi.yaml -o docs/index.html
```

Or validate/lint:

```bash
npx @redocly/cli lint openapi.yaml
```

## 📁 Project Structure

```
.
├── cmd/server/main.go       # Entry point
├── db/migrations/           # dbmate SQL migrations
├── internal/
│   ├── config/              # Environment config
│   ├── db/                  # Database connection
│   ├── handler/             # HTTP handlers
│   ├── httpapi/             # Response/error helpers + request-id
│   ├── logging/             # slog structured logging
│   ├── middleware/          # Auth, request-id, logging, recoverer
│   ├── model/               # GORM models
│   ├── redisclient/         # Redis client wrapper (degraded mode if unset)
│   ├── repository/          # Data access layer
│   ├── router/              # Route setup (/api/v1, /ws)
│   ├── service/             # Business logic (messages, outbox worker)
│   └── ws/                  # WebSocket hub
├── openapi.yaml             # API specification
├── postman/                 # Postman collection
├── Makefile                 # Dev/migration shortcuts
├── go.mod
├── Dockerfile
└── .github/workflows/       # CI/CD pipelines
```

## 🌐 API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | ❌ | Liveness check |
| `GET` | `/readyz` | ❌ | Readiness check (DB + PeerJS) |
| `GET` | `/api/v1/users` | ✅ | List all users |
| `POST` | `/api/v1/users` | ✅ | Create/update current user |
| `GET` | `/api/v1/users/search?q=` | ✅ | Search users |
| `GET` | `/api/v1/chats` | ✅ | List user's chats |
| `POST` | `/api/v1/chats` | ✅ | Create a chat |
| `GET` | `/api/v1/chats/:id` | ✅ | Get chat details |
| `DELETE` | `/api/v1/chats/:id` | ✅ | Delete direct chat |
| `GET` | `/api/v1/messages/:chatId` | ✅ | Get messages |
| `POST` | `/api/v1/messages/:chatId` | ✅ | Send message |
| `POST` | `/api/v1/upload` | ✅ | Upload file |
| `WS` | `/ws` | ❌ | WebSocket connection |

All errors use a consistent shape: `{"error": {"code": "NOT_FOUND", "message": "...", "request_id": "..."}}`. Every response includes an `X-Request-ID` header for tracing.

### Messages (keyset pagination + idempotent send)

`GET /api/v1/messages/:chatId` supports cursor-based (keyset) pagination:

```http
GET /api/v1/messages/:chatId?cursor=31&limit=20
```

- `cursor` - `seq` of the last message you have (from the previous page's `nextCursor`); omit for the newest page
- `limit` - page size, defaults to `50`, max `100`
- Response: `{"messages": [...], "nextCursor": 31}` — empty/`null` `nextCursor` means no older messages

`POST /api/v1/messages/:chatId` accepts an optional `client_message_id`:

```json
{"content": "hi", "client_message_id": "uuid-or-any-unique-string"}
```

Sending the same `client_message_id` twice is a **no-op** (same message returned, no duplicate) — the idempotency key is unique per `(chat_id, client_message_id)`.

### WebSocket (`/ws`)

- Connect with the Google ID token as a query param: `ws://host/ws?token=<google_id_token>`
- Server sends `ping` frames and expects `pong` back (60s idle timeout) to keep dead connections cleaned up
- The server authenticates on upgrade, so the token is never sent as an HTTP header (browsers don't allow custom WS headers)
- Client events: `join-room`, `leave-room`, `send-message`, `resync`, `typing` / `stop-typing`, `call-user`
- Server events: `receive-message`, `message-sent`-style acks are sent as `send-message-error` / `join-room-error`, `resync-messages`, `user-typing`, `user-stop-typing`, `incoming-call`, `error`

Full flow + reliability guarantees are documented in [`docs/message-send-flow.md`](docs/message-send-flow.md).

## 🧪 Tech Stack

**Backend:** Go, Chi, GORM, PostgreSQL, gorilla/websocket, go-oidc (Google), Cloudflare R2, godotenv

**Frontend:** React 19, TypeScript, Vite, Google Identity Services, Native WebSocket, PeerJS, React Router, Lucide, react-easy-crop

**Infrastructure:** Docker, GitHub Actions (CI/CD), Neon (PostgreSQL), Cloudflare R2, Google OAuth, PeerJS

## 🚧 Future Development

- **Unit tests** — Backend Go tests + frontend Vitest
- **Rate limiting** — API rate limit middleware
- **Message search** — Search through chat history
- **Read receipts** — "Seen" status on messages
- **Push notifications** — Web push API or email notifications
- **Typing indicator in chat list** — Shows "typing..." below username
- **Improved online status** — Better presence indicators

## 📄 License

MIT
