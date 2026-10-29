# Notes API (Native Go `net/http`)

A high-performance RESTful Notes API written in Go using only the standard library (`net/http`) with Go 1.22+ routing features.

## Architecture

- **Driver**: Go standard library `net/http` (`http.NewServeMux()` with path matching like `POST /api/v1/notes/{id}/share`)
- **Database**: PostgreSQL (`notes_api` database)
- **Auth Cache**: Redis (DB `0`)
- **Authentication**: JWT (HMAC-SHA256) with Redis refresh token storage, session caching, and revocation blacklist
- **Port**: Container `8080`, mapped to host port **`9933`**

## Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | HTTP listen port |
| `DB_HOST` | `postgres` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL username |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `notes_api` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode |
| `REDIS_HOST` | `redis` | Redis hostname |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_DB` | `0` | Redis database index |
| `JWT_SECRET` | `super-secret-jwt-key-...` | Secret key for JWT signing |
| `JWT_EXPIRATION_HOURS` | `24` | Access token lifespan |
| `JWT_REFRESH_EXPIRATION_HOURS` | `168` | Refresh token lifespan (7 days) in Redis |

## Endpoints

### Auth
- `POST /api/v1/auth/register` - Register user (`username`, `email`, `password`)
- `POST /api/v1/auth/login` - Authenticate user, returns access token + refresh token
- `POST /api/v1/auth/refresh` - Exchange refresh token for new access token and rotated refresh token
- `POST /api/v1/auth/logout` - Blacklist token in Redis (Auth required)
- `GET /api/v1/auth/me` - Get profile of current user (Auth required)

### Notes & Sharing
- `POST /api/v1/notes` - Create note (`title`, `content`) (Auth required)
- `GET /api/v1/notes` - List notes accessible by user (`filter=all|owned|shared`) (Auth required)
- `GET /api/v1/notes/{id}` - Get note details (Auth required, owner or shared recipient)
- `PUT /api/v1/notes/{id}` - Update note (Auth required, owner or shared with `write` permission)
- `DELETE /api/v1/notes/{id}` - Delete note (Auth required, owner only)
- `POST /api/v1/notes/{id}/share` - Share note (`username`, `permission: "read"|"write"`) (Auth required, owner only)
- `GET /api/v1/notes/{id}/shares` - List shared users for note (Auth required, owner only)
- `DELETE /api/v1/notes/{id}/share/{userId}` - Revoke share (Auth required, owner only)
- `GET /api/v1/notes/{id}/access` - Check whether note is shared to a user or owned, and check permissions (Auth required)

### System
- `GET /health` - Health check (DB ping, Redis ping)

## Verification

Run the verification test script from the project root:

```bash
./verify-notes-api.sh 9933
```
