#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "Starting both services..."
docker compose up -d --build

trap 'docker compose up -d restaurant >/dev/null 2>&1 || true' EXIT

echo
echo "POST with dependency up:"
curl -sS -i -X POST http://localhost:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Correlation-ID: demo-up' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}'

echo
echo "Stopping restaurant..."
docker compose stop restaurant

echo
echo "POST with dependency down:"
start_ms=$(date +%s%3N)
curl -sS -i -X POST http://localhost:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Correlation-ID: demo-down' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}'
end_ms=$(date +%s%3N)
echo "elapsed_ms=$((end_ms - start_ms))"

echo
echo "Order service logs:"
docker compose logs --tail=50 order
