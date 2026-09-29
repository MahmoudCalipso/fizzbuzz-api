<p align="center">
  <img src="wevioo-logo.png" alt="Wevioo logo" width="220">
</p>

<h1 align="center">Test Développeur GO</h1>

## FizzBuzz REST API

A production-oriented fizz-buzz REST server written in **Go** with **Gin**, structured
following **Clean Architecture** (layered / hexagonal): **controller → service → repository**,
documented with **Swagger (OpenAPI)**.

Given `int1`, `int2`, `limit`, `str1`, `str2` (sent as a **JSON request body**), it returns the
numbers from 1 to `limit` where multiples of `int1` become `str1`, multiples of `int2` become
`str2`, and multiples of both become `str1str2`. A statistics endpoint reports the most
frequent request and its hit count, plus a generation request history with Excel export.

---

## Quick start

```bash
go mod tidy          # resolves dependencies and generates go.sum (first time only)
make run             # starts the server on :8080

curl -X POST http://localhost:8080/api/v1/fizzbuzz \
  -H "Content-Type: application/json" \
  -d '{"int1":3,"int2":5,"limit":15,"str1":"fizz","str2":"buzz"}'
# ["1","2","fizz","4","buzz","fizz","7","8","fizz","buzz","11","fizz","13","14","fizzbuzz"]

curl http://localhost:8080/api/v1/stats
# {"int1":3,"int2":5,"limit":15,"str1":"fizz","str2":"buzz","hits":1}

curl http://localhost:8080/api/v1/history
curl -OJ http://localhost:8080/api/v1/history/export
```

Swagger UI: **http://localhost:8080/swagger/index.html**
(raw spec: `http://localhost:8080/swagger/doc.json`, also stored in `docs/swagger.json`).

Requirements: Go 1.22+ (Docker optional).

---

## API

### `POST /api/v1/fizzbuzz`

`Content-Type: application/json`

```json
{ "int1": 3, "int2": 5, "limit": 100, "str1": "fizz", "str2": "buzz" }
```

| Field   | Type    | Rules                                 |
|---------|---------|---------------------------------------|
| `int1`  | integer | required, > 0                         |
| `int2`  | integer | required, > 0                         |
| `limit` | integer | required, > 0 and <= `MAX_LIMIT`      |
| `str1`  | string  | required, non-empty, <= 64 characters |
| `str2`  | string  | required, non-empty, <= 64 characters |

The body is limited to 4 KiB.

**200** – JSON array of strings.
**400** – `{"error": "..."}`: malformed JSON, wrong types, missing fields, or invalid values.
**405** – any other HTTP method on this route (e.g. GET).

### `GET /api/v1/stats`

No parameters. Returns the parameters of the most frequent **valid** request and its hits.

**200** – `{"int1":3,"int2":5,"limit":100,"str1":"fizz","str2":"buzz","hits":42}`
**404** – `{"error": "no statistics available yet"}` when no request has been served yet.

If several combinations are tied, the one that reached the top count first is returned.

### `GET /api/v1/history`

Returns successful generation requests, newest first. Identical URL paths and validated
parameters are grouped into one entry; `requested_at` is the UTC time of the latest matching
request, and `hits` is the number of successful requests in that group.

```json
[{"path_url":"/api/v1/fizzbuzz","body":{"int1":3,"int2":5,"limit":100,"str1":"fizz","str2":"buzz"},"requested_at":"2026-09-29T12:00:00Z","hits":2}]
```

### `GET /api/v1/history/export`

Downloads the same history as an Excel `.xlsx` workbook with columns for path URL, JSON
body, last request time (UTC), and hits.

### `GET /healthz`

Liveness/readiness probe. Returns `200 {"status":"ok"}`.

### Errors

All errors share the same shape: `{"error": "<message>"}`. Unexpected errors return
`500 {"error":"internal server error"}`; details are logged, never sent to the client.

---

## Architecture

```
cmd/server/main.go            Composition root: config, wiring, graceful shutdown
docs/                         OpenAPI spec (docs.go registers it, swagger.json)
internal/
├── domain/                   Entities + ports. Zero external dependencies.
│   ├── params.go             Params, validation rules, ValidationError
│   └── stats.go              Stat, ErrNoStats, StatsRepository (port)
├── service/                  Business logic (use cases). Depends only on domain.
│   └── fizzbuzz.go           Generate, MostFrequent
├── repository/               Adapter implementing domain.StatsRepository
│   └── memory_stats.go       Thread-safe in-memory store
├── controller/               HTTP adapter (Gin): routing, JSON binding, error mapping, Swagger annotations
│   ├── controller.go         Request/response DTOs + endpoint handlers
│   └── router.go             Routes, middlewares, Swagger UI
└── config/                   Environment-based configuration
```

Dependency rule: dependencies point **inward**.

```
controller ──► service ──► domain ◄── repository
  (HTTP)     (use cases)   (core)      (storage)
```

- `controller` only translates HTTP <-> use cases. It depends on a small `UseCase`
  interface declared **in the controller package** (consumer-side interface), so it is
  tested with a trivial fake.
- `service` depends on the `domain.StatsRepository` interface, never on a concrete store.
- `repository` implements that interface. Swapping in Redis or PostgreSQL only means
  writing a new adapter and changing one line in `main.go`.

### Design decisions

- **Request body, not URL**: parameters travel in a JSON body (`POST /fizzbuzz`). DTO fields
  are pointers, so a *missing* field is reported as missing while an explicit `0` or `""`
  reaches the domain layer and gets a precise validation message.
- **Stats in O(1)**: the current winner is updated on every write, so `/stats` never
  scans the map.
- **Bounded memory**: the in-memory store tracks at most `MAX_STATS_ENTRIES` distinct
  parameter sets and history request groups. New distinct requests beyond the cap return
  `503 Service Unavailable`; hits for tracked requests continue to be recorded.
- **Bounded input/output**: `limit` is capped by `MAX_LIMIT` (default 10 000), request bodies by 4 KiB.
- **Only valid requests count** towards the statistics.
- **Fast generation**: one slice allocation, `str1+str2` computed once, no per-item
  string formatting other than `strconv.Itoa`.
- **Production hygiene**: HTTP server timeouts, graceful shutdown on SIGINT/SIGTERM,
  panic recovery, structured JSON logs (`log/slog`), Gin release mode by default,
  non-root distroless container.
- **Statistics and history are per instance and reset on restart** (in-memory). For multiple
  replicas or durable history, implement `domain.StatsRepository` with shared persistent storage.

---

## Swagger / OpenAPI

Endpoints are annotated with [swaggo](https://github.com/swaggo/swag) comments in
`internal/controller/controller.go` (general info is in `cmd/server/main.go`).
The spec is served by Swagger UI at `/swagger/index.html`.

After changing an annotation or a DTO, regenerate the spec:

```bash
make swagger     # runs swag init and rewrites docs/docs.go and docs/swagger.json
```

The Swagger UI is always enabled; put it behind authentication or remove the route in
`router.go` if the API must not expose its documentation in production.

---

## Configuration

| Variable            | Default  | Description                                          |
|---------------------|----------|------------------------------------------------------|
| `PORT`              | `8080`   | HTTP port                                            |
| `MAX_LIMIT`         | `10000`  | Largest accepted `limit`                             |
| `MAX_STATS_ENTRIES` | `100000` | Max distinct combinations tracked (`0` = unbounded)  |
| `SHUTDOWN_TIMEOUT`  | `10s`    | Grace period to finish in-flight requests            |
| `GIN_MODE`          | `release`| Set to `debug` for Gin's verbose logs                |

---

## Tests

```bash
make test     # unit + integration tests
make race     # same, with the race detector (recommended in CI)
make cover    # coverage report (coverage.out / coverage.html)
make bench    # benchmarks (fizzbuzz generation, stats recording)
```

| Layer        | What is tested                                                                                           |
|--------------|----------------------------------------------------------------------------------------------------------|
| `domain`     | Validation rules (table-driven, incl. multibyte strings and boundaries)                                  |
| `service`    | Fizz-buzz algorithm (classic, same divisors, divisor 1, limit 1...), recording rules, error propagation, using a fake repository |
| `repository` | Counting, empty state, tie-breaking, memory cap, **concurrent access** (run with `-race`)                |
| `controller` | JSON body handling (malformed, missing fields, wrong types, oversized), status codes, error mapping, no leak of internal errors, 405 on GET, Swagger spec served, using a fake use case |
| `controller` | End-to-end test with the real service and repository behind the real router                              |
| `config`     | Defaults, overrides, invalid values                                                                      |

---

## Docker

```bash
go mod tidy                     # generates go.sum, required by the Dockerfile
docker build -t fizzbuzz-api .
docker run --rm -p 8080:8080 -e MAX_LIMIT=5000 fizzbuzz-api
```

---

## Possible improvements

- Persistent / shared statistics store (Redis, PostgreSQL) behind `StatsRepository`.
- Rate limiting and authentication middlewares.
- Prometheus metrics and OpenTelemetry tracing.
