# Dual Notes API: Go Native (`net/http`) & Gin Framework

A production-ready implementation of two identical Notes APIs in Go, demonstrating clean architecture, JWT authentication, PostgreSQL persistence, and Redis auth caching:

1. **`notes-api`** (`./notes-api`): Native Go implementation using only the standard library `net/http` (with Go 1.22+ routing features). Exposed on port **`9933`**.
2. **`notes-api-gin`** (`./notes-api-gin`): Framework implementation using `github.com/gin-gonic/gin`. Exposed on port **`9944`**.

Both applications share the same PostgreSQL database instance (with isolated databases: `notes_api` and `notes_api_gin`) and the same Redis cache instance (with isolated Redis DB indices: `DB 0` and `DB 1`). PostgreSQL and Redis remain internal to the Docker network with no host port collisions.

---

## Directory Structure

```
go-apm-agent/
├── docker-compose.yml           # Multi-container orchestration
├── init-scripts/
│   └── 01-init-dbs.sql          # Auto-initializes notes_api & notes_api_gin DBs
├── notes-api/                   # Native Go net/http API (Port 9933)
│   ├── README.md
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── config/                  # Environment & connection configuration
│   ├── database/                # PostgreSQL pool & auto-migrations
│   ├── cache/                   # Redis session & blacklist caching
│   ├── models/                  # Domain structs & DTOs
│   ├── repository/              # PostgreSQL data access layer
│   ├── service/                 # Business logic & JWT operations
│   ├── middleware/              # Auth, CORS, Logger middlewares
│   └── handlers/                # HTTP request handlers
├── notes-api-gin/               # Gin Framework API (Port 9944)
│   ├── README.md
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── config/
│   ├── database/
│   ├── cache/
│   ├── models/
│   ├── repository/
│   ├── service/
│   ├── middleware/
│   └── handlers/
├── verify-notes-api.sh          # Full test suite for notes-api (2xx & 4xx assertions)
└── verify-notes-api-gin.sh      # Full test suite for notes-api-gin (2xx & 4xx assertions)
```

---

## Port Allocation & Environment

| Service | Internal Port | Host Port | Database / Cache Index |
| :--- | :--- | :--- | :--- |
| **`notes-api`** (native) | `8080` | **`9933`** | PostgreSQL DB: `notes_api` \| Redis DB: `0` |
| **`notes-api-gin`** (gin) | `8081` | **`9944`** | PostgreSQL DB: `notes_api_gin` \| Redis DB: `1` |
| **`postgres`** | `5432` | Internal only | Isolated databases auto-created on start |
| **`redis`** | `6379` | Internal only | Shared Redis instance with isolated DB indices |

---

## Running the Applications

To build and start all 4 services:

```bash
docker compose up -d
```

Check running containers:

```bash
docker compose ps
```

View application logs:

```bash
docker compose logs -f notes-api notes-api-gin
```

---

## Running the Verification Suites

Each API comes with an automated verification test script that rigorously exercises all endpoints, validating both **2xx success** (200, 201) and **4xx client error** (400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found) scenarios:

```bash
# Verify Native Go API on port 9933
./verify-notes-api.sh

# Verify Gin API on port 9944
./verify-notes-api-gin.sh
```

---

## API Summary Table

All protected endpoints require the header: `Authorization: Bearer <token>`.

| Method | Endpoint | Access | Description | Success / Error Codes |
| :--- | :--- | :--- | :--- | :--- |
| `GET` | `/health` | Public | Service health & database/cache status | `200` |
| `POST` | `/api/v1/auth/register` | Public | Register user (`username`, `email`, `password`) | `201`, `400` |
| `POST` | `/api/v1/auth/login` | Public | Authenticate user, issue JWT & cache in Redis | `200`, `400`, `401` |
| `POST` | `/api/v1/auth/logout` | Protected | Blacklist token in Redis auth cache | `200`, `401` |
| `GET` | `/api/v1/auth/me` | Protected | Retrieve authenticated user profile | `200`, `401` |
| `POST` | `/api/v1/notes` | Protected | Create new note (`title`, `content`) | `201`, `400`, `401` |
| `GET` | `/api/v1/notes` | Protected | List accessible notes (`filter=all\|owned\|shared`) | `200`, `401` |
| `GET` | `/api/v1/notes/:id` | Protected | Get note details (owner or shared recipient) | `200`, `401`, `403`, `404` |
| `PUT` | `/api/v1/notes/:id` | Protected | Update note (owner or shared with `write` permission) | `200`, `400`, `401`, `403`, `404` |
| `DELETE`| `/api/v1/notes/:id` | Protected | Delete note (owner only) | `200`, `401`, `403`, `404` |
| `POST` | `/api/v1/notes/:id/share`| Protected (Owner) | Share note (`username` or `user_id`, `permission: "read"\|"write"`) | `200`, `400`, `401`, `403`, `404` |
| `GET` | `/api/v1/notes/:id/shares`| Protected (Owner) | List all users with access to note | `200`, `401`, `403`, `404` |
| `DELETE`| `/api/v1/notes/:id/share/:userId` | Protected (Owner) | Revoke a user's share access | `200`, `401`, `403`, `404` |
| `GET` | `/api/v1/notes/:id/access`| Protected | Authorization check: verify if note is shared to a user or owned | `200`, `401`, `403`, `404` |
