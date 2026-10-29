# Dual Notes API: Go Native (`net/http`) & Gin Framework

A production-ready implementation of two identical Notes APIs in Go, demonstrating clean architecture, JWT authentication with Redis refresh token storage, and PostgreSQL database persistence:

1. **`notes-api`** (`./notes-api`): Native Go implementation using only the standard library `net/http` (with Go 1.22+ routing features). Exposed on port **`9933`**.
2. **`notes-api-gin`** (`./notes-api-gin`): Framework implementation using `github.com/gin-gonic/gin`. Exposed on port **`9944`**.

Both applications share the same PostgreSQL database instance (with isolated databases: `notes_api` and `notes_api_gin`) and the same Redis cache instance (with isolated Redis DB indices: `DB 0` and `DB 1`). PostgreSQL and Redis remain internal to the Docker network with no host port collisions.

---

## Directory Structure

```
go-apm-agent/
├── .env                         # Root environment configuration
├── .env.example                 # Root environment template
├── docker-compose.yml           # Multi-container orchestration
├── init-scripts/
│   └── 01-init-dbs.sql          # Auto-initializes notes_api & notes_api_gin DBs
├── notes-api/                   # Native Go net/http API (Port 9933)
│   ├── .env.example             # Service-level environment template
│   ├── README.md
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── config/                  # Environment & connection configuration
│   ├── database/                # PostgreSQL pool & auto-migrations
│   ├── cache/                   # Redis session, refresh token & blacklist caching
│   ├── models/                  # Domain structs & DTOs
│   ├── repository/              # PostgreSQL data access layer
│   ├── service/                 # Business logic, JWT & refresh token operations
│   ├── middleware/              # Auth, CORS, Logger middlewares
│   └── handlers/                # HTTP request handlers
├── notes-api-gin/               # Gin Framework API (Port 9944)
│   ├── .env.example             # Service-level environment template
│   ├── README.md
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── config/                  # Environment & connection configuration
│   ├── database/                # PostgreSQL pool & auto-migrations
│   ├── cache/                   # Redis session, refresh token & blacklist caching
│   ├── models/                  # Domain structs & DTOs
│   ├── repository/              # PostgreSQL data access layer
│   ├── service/                 # Business logic, JWT & refresh token operations
│   ├── middleware/              # Auth, CORS middlewares
│   └── handlers/                # HTTP request handlers
├── verify-notes-api.sh          # Automated test suite for notes-api (40 tests)
└── verify-notes-api-gin.sh      # Automated test suite for notes-api-gin (40 tests)
```

---

## Storage & Authentication Architecture

### 1. PostgreSQL Persistence
All persistent domain entities are stored in PostgreSQL:
- **`users` table**: ID, username, email, bcrypt password hash, created_at, updated_at.
- **`notes` table**: ID, user_id (foreign key to users), title, content, created_at, updated_at.
- **`note_shares` table**: ID, note_id (foreign key to notes), shared_with_user_id (foreign key to users), permission (`read` or `write`), created_at.

### 2. Redis Authentication & Token Storage
Redis provides high-performance token and session management:
- **Refresh Tokens (`auth:refresh:<token>`)**: Stores the user identity mapped to the refresh token with a 7-day TTL (configured via `JWT_REFRESH_EXPIRATION_HOURS`). Tokens are rotated upon every refresh.
- **Session Cache (`auth:session:<jti>`)**: Caches decoded JWT user identity with the token's lifetime to avoid redundant database lookups on every request.
- **Token Blacklist (`auth:blacklist:<jti>`)**: Instantly revokes JWT access tokens on logout until their original expiration.

---

## API Endpoints

| Method | Path | Auth Required | Description |
|---|---|---|---|
| `GET` | `/health` | No | Service, PostgreSQL & Redis health check |
| `POST` | `/api/v1/auth/register` | No | Register a new user (stores to PostgreSQL) |
| `POST` | `/api/v1/auth/login` | No | Login; returns JWT access token & Redis refresh token |
| `POST` | `/api/v1/auth/refresh` | No | Exchange refresh token for new access token & rotated refresh token |
| `POST` | `/api/v1/auth/logout` | Yes | Invalidate token in Redis cache & blacklist |
| `GET` | `/api/v1/auth/me` | Yes | Get current user profile |
| `POST` | `/api/v1/notes` | Yes | Create a note (persisted to PostgreSQL) |
| `GET` | `/api/v1/notes` | Yes | List owned and shared notes |
| `GET` | `/api/v1/notes/:id` | Yes | Get note by ID (access controlled) |
| `PUT` | `/api/v1/notes/:id` | Yes | Update note (owner or write permission) |
| `DELETE` | `/api/v1/notes/:id` | Yes | Delete note (owner only) |
| `POST` | `/api/v1/notes/:id/share` | Yes | Share note with another user (read/write) |
| `GET` | `/api/v1/notes/:id/shares` | Yes | List all users a note is shared with |
| `DELETE` | `/api/v1/notes/:id/share/:userId` | Yes | Revoke user access to a note |
| `GET` | `/api/v1/notes/:id/access` | Yes | Check authorization details for note |

---

## Running with Docker Compose

```bash
# Start all services
docker compose up -d --build

# Check status
docker compose ps
```

## Running Verification Suites

```bash
# Test native net/http API (Port 9933)
./verify-notes-api.sh 9933

# Test Gin API (Port 9944)
./verify-notes-api-gin.sh 9944
```
