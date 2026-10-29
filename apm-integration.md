# Elastic APM Integration Guide (Go)

This guide shows how to add [Elastic APM](https://www.elastic.co/guide/en/apm/agent/go/current/index.html) tracing to a Go web service so that **every HTTP request, PostgreSQL query and Redis command** is captured. It covers both styles used in this repository:

- **Plain Go** (`net/http`, Go 1.22+ `ServeMux`): see [`notes-api`](./notes-api)
- **Gin framework** (`github.com/gin-gonic/gin`): see [`notes-api-gin`](./notes-api-gin)

The goal is to keep code changes small. Business logic, handlers, services and repositories stay untouched.

---

## What Gets Captured

| Component | APM concept | Example in Kibana |
|---|---|---|
| Incoming HTTP request | **Transaction** | `GET /api/v1/notes/{id}` (plain) / `GET /api/v1/notes/:id` (Gin) |
| PostgreSQL query | **Span** (`db/postgresql`) | `SELECT FROM notes` |
| Redis command | **Span** (`db/redis`) | `GET`, `SET`, `EXISTS`, `DEL` |

Spans are nested under the transaction that caused them, so one request shows its full timeline:

```
GET /api/v1/auth/me                      12.4 ms   (transaction)
├── EXISTS                                0.6 ms   (redis: blacklist check)
└── GET                                   0.5 ms   (redis: session cache)

POST /api/v1/notes                        8.1 ms   (transaction)
└── INSERT INTO notes                     3.2 ms   (postgresql)
```

---

## Prerequisite: Pass `context.Context` Everywhere

APM links a DB or Redis span to its HTTP transaction **through the request context**. Instrumentation only nests correctly if the request context flows down to the data layer:

```go
// handler
note, err := h.service.GetNote(r.Context(), id)      // plain Go
note, err := h.service.GetNote(c.Request.Context(), id) // Gin

// repository: always use the *Context variants
r.db.QueryRowContext(ctx, query, id)   // ✅ captured as child span
r.db.QueryRow(query, id)               // ❌ no parent, span is dropped

// redis: go-redis always takes ctx
r.client.Get(ctx, key)                 // ✅
```

If you pass `context.Background()` instead, the call still works but won't show up under the request.

---

## Step 1: Install Packages

**Both styles:**

```bash
go get go.elastic.co/apm/v2 \
       go.elastic.co/apm/module/apmhttp/v2 \
       go.elastic.co/apm/module/apmsql/v2
```

**Gin only (additional):**

```bash
go get go.elastic.co/apm/module/apmgin/v2
```

> Redis needs no extra package: the hook in Step 4 uses only `go.elastic.co/apm/v2`.

---

## Step 2: Instrument HTTP (the Transaction)

This is the only step that differs between plain Go and Gin.

### Option A: Plain Go (`net/http`)

Wrap the **outermost** handler with `apmhttp.Wrap` so the transaction covers all middleware.

`apmhttp` names transactions by the raw URL path (`GET /api/v1/notes/42`), which creates one group per ID. A small wrapper around the mux renames the transaction to the matched route pattern after routing. `r.Pattern` is set by `ServeMux` since Go 1.23.

```go
import (
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
)

mux := http.NewServeMux()
mux.HandleFunc("GET /api/v1/notes/{id}", noteHandler.GetNote)
// ... other routes ...

// Name APM transactions by route pattern (e.g. "GET /api/v1/notes/{id}")
// instead of the raw path, so requests group per endpoint.
routed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	mux.ServeHTTP(w, r)
	if tx := apm.TransactionFromContext(r.Context()); tx != nil && r.Pattern != "" {
		tx.Name = r.Pattern
	}
})

// Global Middlewares: APM (outermost), Logger and CORS
handler := apmhttp.Wrap(middleware.Logger(middleware.CORS(routed)))

server := &http.Server{Addr: ":8080", Handler: handler}
```

### Option B: Gin

Register `apmgin.Middleware` **first**. It names transactions by the route template (`c.FullPath()`) automatically and records panics as errors.

```go
import "go.elastic.co/apm/module/apmgin/v2"

router := gin.New()
router.Use(apmgin.Middleware(router)) // APM first: traces every request by route
router.Use(gin.Recovery())
router.Use(gin.Logger())
router.Use(middleware.CORS())
```

### Plain Go vs Gin at a Glance

| | Plain Go | Gin |
|---|---|---|
| Package | `apmhttp` | `apmgin` |
| Hook point | Wrap the root `http.Handler` | `router.Use(...)` as first middleware |
| Route-based transaction names | Needs the 5-line `r.Pattern` wrapper | Built in |
| Context for downstream calls | `r.Context()` | `c.Request.Context()` |

---

## Step 3: Instrument PostgreSQL (`database/sql` + `lib/pq`)

Identical for both styles. Replace the driver import and `sql.Open`. Everything else (`*sql.DB`, pooling, `QueryContext`, transactions) stays the same.

```diff
 import (
 	"database/sql"
-	_ "github.com/lib/pq"
+	"go.elastic.co/apm/module/apmsql/v2"
+	_ "go.elastic.co/apm/module/apmsql/v2/pq" // APM-instrumented lib/pq driver
 )

-db, err := sql.Open("postgres", dsn)
+db, err := apmsql.Open("postgres", dsn)
```

Each `QueryContext` / `ExecContext` / `QueryRowContext` call becomes a span with the SQL statement attached. Using `pgx` or MySQL instead? `apmsql` ships drivers for those too (`apmsql/pgxv4`, `apmsql/mysql`, …) and works the same way.

---

## Step 4: Instrument Redis (go-redis v9)

Identical for both styles. Elastic's official module (`apmgoredisv8`) supports only **go-redis v8**. For **v9**, add a small hook using go-redis's built-in hook interface.

### 4.1 Add the hook file: `cache/apm_hook.go`

```go
package cache

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.elastic.co/apm/v2"
)

// apmHook records every Redis command as an Elastic APM span under the
// current transaction (Elastic has no official module for go-redis v9).
type apmHook struct{}

func (apmHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (apmHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		span, ctx := startRedisSpan(ctx, strings.ToUpper(cmd.Name()))
		defer span.End()
		err := next(ctx, cmd)
		markRedisOutcome(span, err)
		return err
	}
}

func (apmHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		span, ctx := startRedisSpan(ctx, "PIPELINE")
		defer span.End()
		err := next(ctx, cmds)
		markRedisOutcome(span, err)
		return err
	}
}

func startRedisSpan(ctx context.Context, name string) (*apm.Span, context.Context) {
	span, ctx := apm.StartSpan(ctx, name, "db.redis")
	span.Context.SetDatabase(apm.DatabaseSpanContext{Type: "redis", Statement: name})
	return span, ctx
}

// redis.Nil (key not found) is a normal cache miss, not a failure.
func markRedisOutcome(span *apm.Span, err error) {
	if err != nil && !errors.Is(err, redis.Nil) {
		span.Outcome = "failure"
	}
}
```

### 4.2 Register it (one line)

```go
rdb := redis.NewClient(&redis.Options{
	Addr:     cfg.RedisAddr(),
	Password: cfg.RedisPassword,
	DB:       cfg.RedisDB,
})
rdb.AddHook(apmHook{})
```

Notes:
- Only the command name (`GET`, `SET`, …) is recorded, **not keys or values**, so tokens and session data stored in Redis never leave the service.
- Calls made outside a request (e.g. the startup `Ping`) have no transaction and are silently skipped.
- On **go-redis v8**, use the official module instead: `rdb.AddHook(apmgoredisv8.NewHook())`.

---

## Step 5: Flush on Shutdown

The agent buffers events and sends them in the background. Flush once after the HTTP server stops, so the last requests aren't lost:

```go
if err := server.Shutdown(shutdownCtx); err != nil {
	log.Fatalf("Server forced to shutdown: %v", err)
}

// Send any buffered APM events before exiting
apm.DefaultTracer().Flush(nil)
```

---

## Step 6: Configure via Environment Variables

No config code is needed. The agent reads `ELASTIC_APM_*` variables at startup.

| Variable | Purpose | Example |
|---|---|---|
| `ELASTIC_APM_SERVER_URL` | APM Server endpoint | `http://apm-server:8200` |
| `ELASTIC_APM_SECRET_TOKEN` | Auth token for APM Server | *(from your Elastic deployment)* |
| `ELASTIC_APM_SERVICE_NAME` | Name shown in Kibana | `notes-api` / `notes-api-gin` |
| `ELASTIC_APM_ENVIRONMENT` | Environment filter in Kibana | `staging` |
| `ELASTIC_APM_TRANSACTION_SAMPLE_RATE` | Fraction of requests traced (default `1.0`) | `0.2` |
| `ELASTIC_APM_LOG_LEVEL` / `ELASTIC_APM_LOG_FILE` | Agent debug logs | `debug` / `stderr` |

In this repo, [`docker-compose.yml`](./docker-compose.yml) sets them per service:

```yaml
    environment:
      # Elastic APM agent
      ELASTIC_APM_SERVICE_NAME: notes-api
      ELASTIC_APM_SERVER_URL: ${ELASTIC_APM_SERVER_URL:-http://apm-server:8200}
      ELASTIC_APM_SECRET_TOKEN: ${ELASTIC_APM_SECRET_TOKEN:-}
      ELASTIC_APM_ENVIRONMENT: ${APP_ENV:-staging}
```

The real URL and token go in the root `.env` (git-ignored). [`.env.example`](./.env.example) has placeholders:

```dotenv
ELASTIC_APM_SERVER_URL=http://apm-server:8200
ELASTIC_APM_SECRET_TOKEN=
```

> Never commit the secret token. Keep it in `.env` or your secret manager.

---

## Verifying the Integration

1. **Check the APM Server is reachable** from where the service runs:

   ```bash
   curl -H "Authorization: Bearer $ELASTIC_APM_SECRET_TOKEN" $ELASTIC_APM_SERVER_URL/
   # {"version":"8.x.x","publish_ready":true,...}
   ```

2. **Start the services** and send some traffic:

   ```bash
   docker compose up -d --build
   ./verify-notes-api.sh
   ./verify-notes-api-gin.sh
   ```

3. **(Optional) Watch the agent send data** by temporarily enabling debug logs:

   ```bash
   docker compose run --rm -p 9955:8080 \
     -e ELASTIC_APM_LOG_LEVEL=debug -e ELASTIC_APM_LOG_FILE=stderr notes-api
   ```

   You should see lines like:

   ```json
   {"level":"debug","message":"sent request with 5 transactions, 13 spans, 0 errors, 0 metricsets"}
   ```

4. **Open Kibana → Observability → APM → Services**. `notes-api` and `notes-api-gin` appear within ~30 seconds. Open any transaction to see its PostgreSQL and Redis spans in the waterfall.

### Quick Sanity Check: Redis Spans

`GET /api/v1/auth/me` with a valid token is served entirely from Redis (blacklist check + session cache). Its trace should show **exactly 2 Redis spans and no SQL**. If you see 0 spans, the Redis hook is not registered or `ctx` is not being passed through.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Service not visible in Kibana | Wrong `ELASTIC_APM_SERVER_URL`/token, or server unreachable from the container. Enable debug logs (above). |
| Transactions appear but no DB/Redis spans | Data layer uses `context.Background()` or non-`Context` SQL methods. |
| Native transactions named `GET /api/v1/notes/42` | The `r.Pattern` rename wrapper is missing, or Go < 1.23. |
| Gin transactions missing for some middleware | `apmgin.Middleware` is not the first `router.Use(...)`. |
| Last requests before shutdown missing | `apm.DefaultTracer().Flush(nil)` not called. |
| Redis cache misses shown as failures | Hook does not exclude `redis.Nil` (see `markRedisOutcome`). |

---

## Summary Checklist

| # | Change | Plain Go | Gin |
|---|---|---|---|
| 1 | `go get` APM modules | `apm`, `apmhttp`, `apmsql` | + `apmgin` |
| 2 | HTTP transaction | `apmhttp.Wrap(handler)` + `r.Pattern` rename | `router.Use(apmgin.Middleware(router))` |
| 3 | PostgreSQL spans | `apmsql/pq` import + `apmsql.Open` | same |
| 4 | Redis spans | `cache/apm_hook.go` + `rdb.AddHook(apmHook{})` | same |
| 5 | Flush on shutdown | `apm.DefaultTracer().Flush(nil)` | same |
| 6 | Config | `ELASTIC_APM_*` env vars | same |
