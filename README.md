# Assignment 01. Containerized Order and Restaurant Services

Two isolated Go services communicating over HTTP. The Order Service never calls Restaurant code directly; it performs a real HTTP request with a bounded timeout and maps failures to documented HTTP responses.

## Layout

```text
Assignment-1/
├── api/examples.md
├── docs/
├── scripts/outage-demo.sh
├── services/
│   ├── order/
│   └── restaurant/
├── docker-compose.yml
└── README.md
```

## Run with Docker

```bash
docker compose up -d --build
curl -sS http://localhost:8081/healthz
curl -sS http://localhost:8082/healthz
```

The Order Service container uses `RESTAURANT_URL=http://restaurant:8081` (Compose service name), not `localhost`.

## Run locally without Docker

```bash
export PATH="$HOME/.local/go/bin:$PATH"

cd services/restaurant
PORT=8081 go run . &
cd ../order
RESTAURANT_URL=http://localhost:8081 PORT=8082 go run . &
```

## Tests

```bash
export PATH="$HOME/.local/go/bin:$PATH"
(cd services/restaurant && go test -race ./...)
(cd services/order && go test -race ./...)
```

Test cases cover success, health, malformed JSON, invalid quantity, missing restaurant, missing item, dependency outage with bounded latency, order lookup, and 20 concurrent POSTs producing unique IDs.

## Outage experiment

```bash
chmod +x scripts/outage-demo.sh
./scripts/outage-demo.sh
```

It starts both containers, submits a successful order, stops the Restaurant Service, submits another order, records the elapsed time and HTTP response, and prints correlation-ID-tagged logs. A local run is saved in `docs/outage-log.txt`.

## API contract

- `POST /orders`: `{ "restaurant_id": "r1", "items": [{ "menu_item_id": "m1", "quantity": 2 }] }` → `{ "order_id", "correlation_id", "status", "created_at", ... }`
- `GET /orders/{id}` → order JSON or `404`
- `GET /restaurants` and `GET /restaurants/{id}` → catalogue JSON or `404`
- Errors: `400` bad input, `404` missing entity, `503` dependency unavailable

See `api/examples.md` for full request/response examples.

## Implementation notes

- Seed data: 3 restaurants, 9 menu items.
- Order state is an in-memory `map` guarded by `sync.RWMutex` (A1 scope).
- `order_id` and `correlation_id` are 128-bit random hex values from `crypto/rand`.
- Correlation IDs are accepted from `X-Correlation-ID`, echoed in response headers, and included in logs.
- No third-party Go modules are used; both Dockerfiles run as a non-root user.
