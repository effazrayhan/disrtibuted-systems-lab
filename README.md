# Assignment 01 — FastAPI Implementation

This branch (`lab-1-fastapi`) contains the Python FastAPI version of the assignment. The original Go version is on `lab-1`.

Two independent services talk over real HTTP:

| Service | Port | Job |
|---|---:|---|
| Restaurant | `8081` | Serves the restaurant/menu catalogue |
| Order | `8082` | Validates orders by calling Restaurant, then stores them |

The Order Service does **not** import Restaurant code. It calls `GET /restaurants/{id}` over HTTP with a short timeout.

---

## 1. Quick start (recommended)

From this folder:

```bash
./run.sh
```

`run.sh` will:

1. build both Docker images,
2. start both containers on a private Docker network,
3. wait until both `/healthz` endpoints answer,
4. print ready-to-copy commands.

Check that everything is alive:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz
```

Expected response from both:

```json
{"status":"ok"}
```

---

## 2. Manual Docker commands

If you do not want to use `run.sh`:

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f order
docker compose down
```

If Docker says `permission denied`, either open a new terminal after the group change or add `sudo`:

```bash
sudo docker compose down
```

---

## 3. Try the API

List restaurants:

```bash
curl http://localhost:8081/restaurants
```

Create an order:

```bash
curl -X POST http://localhost:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Correlation-ID: demo-001' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":2}]}'
```

You will get a response like:

```json
{
  "order_id": "14c2bd39d9694dc8a71bc253a61850fe",
  "correlation_id": "demo-001",
  "status": "created",
  "created_at": "2026-11-18T09:57:21.602Z",
  "restaurant_id": "r1",
  "items": [{"menu_item_id": "m1", "quantity": 2}]
}
```

Copy the `order_id` and fetch the order:

```bash
curl http://localhost:8082/orders/14c2bd39d9694dc8a71bc253a61850fe
```

Orders are stored in memory only, so they disappear when the Order container restarts. This is expected for Assignment 01.

More examples are in [`api/examples.md`](api/examples.md).

---

## 4. Run the tests

You need Python 3.12+.

```bash
python3 -m venv .venv
source .venv/bin/activate

pip install -r services/restaurant/requirements.txt
pip install -r services/order/requirements.txt
pip install -r requirements-dev.txt

(cd services/restaurant && pytest -q)
(cd services/order && pytest -q)
```

Current result: **3 restaurant tests + 11 order tests pass**.

The tests cover success, health, malformed JSON, missing fields, invalid quantity, duplicate item, missing restaurant, missing item, dependency outage with bounded latency, order lookup, and 20 concurrent orders producing 20 unique IDs.

---

## 5. Outage experiment

```bash
./scripts/outage-demo.sh
```

This script:

1. starts both services,
2. creates a successful order,
3. stops the Restaurant container,
4. tries another order and shows `503 Service Unavailable`,
5. prints correlation-ID-tagged logs.

A saved run is in [`docs/outage-log.txt`](docs/outage-log.txt).

---

## 6. Project structure

```text
Assignment-1/
├── api/examples.md
├── docs/outage-log.txt
├── requirements-dev.txt
├── scripts/outage-demo.sh
├── services/
│   ├── order/
│   │   ├── app.py              # FastAPI order service
│   │   ├── Dockerfile
│   │   ├── requirements.txt
│   │   └── tests/
│   └── restaurant/
│       ├── app.py              # FastAPI restaurant service
│       ├── Dockerfile
│       ├── requirements.txt
│       └── tests/
├── docker-compose.yml
├── run.sh
└── README.md
```

---

## 7. Important environment variables

| Variable | Used by | Default | Meaning |
|---|---|---:|---|
| `PORT` | both | `8081` / `8082` | Listening port |
| `RESTAURANT_URL` | order | required | Base URL of Restaurant Service |
| `RESTAURANT_TIMEOUT_MS` | order | `2000` | Max wait for Restaurant before `503` |

Inside Docker, Compose sets `RESTAURANT_URL=http://restaurant:8081`. `localhost` would mean the Order container itself, so it must not be used there.
