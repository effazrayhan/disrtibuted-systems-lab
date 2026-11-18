# Order Service

Creates orders after validating restaurant and item IDs over HTTP with the Restaurant Service.

## Run locally

```bash
RESTAURANT_URL=http://localhost:8081 PORT=8082 go run .
```

`RESTAURANT_URL` is required. `RESTAURANT_TIMEOUT_MS` defaults to 2000.

## Endpoints

- `GET /healthz`
- `POST /orders`
- `GET /orders/{id}`

## Errors

- `400` malformed JSON, missing fields, invalid quantity, duplicate item
- `404` restaurant or menu item not found, order not found
- `503` Restaurant Service unavailable or timed out
