# Assignment 01 — Containerized Order and Restaurant Services

Two independent FastAPI services communicating over HTTP (no direct imports).

- **Restaurant Service** (`8081`): `GET /healthz`, `GET /restaurants`, `GET /restaurants/{id}` (seed: 3 restaurants, 9 items)
- **Order Service** (`8082`): `GET /healthz`, `POST /orders`, `GET /orders/{id}` (validates via `RESTAURANT_URL`, in-memory store)

## Run with Docker

```bash
docker compose up -d --build
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz
# stop: docker compose down
```

`docker-compose.yml` sets `RESTAURANT_URL=http://restaurant:8081` for the Order container.

## API examples

See [`api/examples.md`](api/examples.md).

## Automated tests (14 cases)

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r services/restaurant/requirements.txt -r services/order/requirements.txt
pytest -q services/restaurant/tests services/order/tests
```

Covers health, catalogue, create/fetch order, malformed/missing/invalid/duplicate input, missing restaurant/item, bounded dependency outage (`503`), and 20 concurrent POSTs producing 20 unique IDs.

## Outage experiment

Example logs: [`docs/outage-log.txt`](docs/outage-log.txt) (Restaurant stopped → `POST /orders` returns bounded `503 dependency_unavailable`).

## Structure

```text
api/examples.md
docker-compose.yml
docs/outage-log.txt
services/order/{app.py,Dockerfile,requirements.txt,tests/test_order.py}
services/restaurant/{app.py,Dockerfile,requirements.txt,tests/test_restaurant.py}
README.md
```
