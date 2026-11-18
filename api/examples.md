# API examples

## Restaurant Service

```bash
curl -sS http://localhost:8081/healthz
curl -sS http://localhost:8081/restaurants
curl -sS http://localhost:8081/restaurants/r1
```

Example `GET /restaurants/r1` response:

```json
{
  "id": "r1",
  "name": "Dhaka Kitchen",
  "items": [
    { "id": "m1", "name": "Chicken Biryani", "price_cents": 35000, "available": true },
    { "id": "m2", "name": "Seekh Kebab", "price_cents": 28000, "available": true },
    { "id": "m3", "name": "Mango Lassi", "price_cents": 12000, "available": true }
  ]
}
```

## Order Service

```bash
curl -sS -X POST http://localhost:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Correlation-ID: demo-001' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":2}]}'
```

Example `201 Created` response:

```json
{
  "order_id": "b7f4c1a28d9e4f6b8c3a5e7d9f1b2c40",
  "correlation_id": "demo-001",
  "status": "created",
  "created_at": "2026-11-18T12:34:56.789Z",
  "restaurant_id": "r1",
  "items": [{ "menu_item_id": "m1", "quantity": 2 }]
}
```

Retrieve the order:

```bash
curl -sS http://localhost:8082/orders/<order_id>
```

## Error examples

```bash
curl -sS -i -X POST http://localhost:8082/orders \
  -H 'Content-Type: application/json' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":0}]}'
```

```json
{ "error": "invalid_item" }
```

With the Restaurant Service stopped, the same valid request returns:

```json
{ "error": "dependency_unavailable" }
```
