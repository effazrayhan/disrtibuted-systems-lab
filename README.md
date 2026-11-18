# Assignment 01. Containerized Order and Restaurant Services

This branch implements the assignment with **Python FastAPI** instead of Go. Two isolated services communicate over HTTP. The Order Service never calls Restaurant code directly; it performs a real HTTP request with a bounded timeout and maps failures to documented HTTP responses.

## Layout

```text
Assignment-1/
├── api/examples.md
├── docs/
├── requirements-dev.txt
├── scripts/outage-demo.sh
├── services/
│   ├── order/
│   │   ├── app.py
│   │   ├── Dockerfile
│   │   ├── requirements.txt
│   │   └── tests/
│   └── restaurant/
│       ├── app.py
│       ├── Dockerfile
│       ├── requirements.txt
│       └── tests/
├── docker-compose.yml
├── run.sh
└── README.md
```

## Run with Docker

```bash
docker compose up -d --build
curl -sS http://localhost:8081/healthz
curl -sS http://localhost:8082/healthz
```

Or simply:

```bash
./run.sh
```

The Order Service container uses `RESTAURANT_URL=http://restaurant:8081` (Compose service name), not `localhost`.

## Run locally without Docker

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r services/restaurant/requirements.txt
pip install -r services/order/requirements.txt

PORT=8081 python services/restaurant/app.py &
RESTAURANT_URL=http://localhost:8081 PORT=8082 python services/order/app.py &
```

## Tests

```bash
source .venv/bin/activate
pip install -r requirements-dev.txt

(cd services/restaurant && pytest -q)
(cd services/order && pytest -q)
```

Tests cover health, success, malformed JSON, missing fields, invalid quantity, duplicate item, missing restaurant, missing item, dependency outage with bounded latency, order lookup, and 20 concurrent POSTs producing unique IDs.

## Outage experiment

```bash
./scripts/outage-demo.sh
```

It starts both containers, submits a successful order, stops the Restaurant Service, submits another order, records the elapsed time and HTTP response, and prints correlation-ID-tagged logs. A run is saved in `docs/outage-log.txt`.

## API contract

- `POST /orders`: `{ "restaurant_id": "r1", "items": [{ "menu_item_id": "m1", "quantity": 2 }] }` → `{ "order_id", "correlation_id", "status", "created_at", ... }`
- `GET /orders/{id}` → order JSON or `404`
- `GET /restaurants` and `GET /restaurants/{id}` → catalogue JSON or `404`
- Errors: `400` bad input, `404` missing entity, `503` dependency unavailable

See `api/examples.md` for full request/response examples.

## Implementation notes

- Seed data: 3 restaurants, 9 menu items.
- Order state is an in-memory `dict` guarded by `asyncio.Lock` (A1 scope).
- `order_id` and `correlation_id` are random 128-bit hex values from `uuid.uuid4().hex`.
- Correlation IDs are accepted from `X-Correlation-ID`, echoed in response headers, and included in logs.
- Both Dockerfiles use `python:3.12-slim`, install only runtime dependencies, and run as a non-root user.
