#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
fi

"${DOCKER[@]}" compose up -d --build

ready=0
for _ in $(seq 1 30); do
  if curl -fsS http://localhost:8081/healthz >/dev/null 2>&1 && curl -fsS http://localhost:8082/healthz >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done

if [ "$ready" -ne 1 ]; then
  "${DOCKER[@]}" compose logs
  exit 1
fi

printf '%s\n' 'Services are up.'
printf '%s\n' 'Restaurant: http://localhost:8081/restaurants'
printf '%s\n' 'Order:      http://localhost:8082/healthz'
printf '%s\n' 'Create order:'
printf '%s\n' "curl -X POST http://localhost:8082/orders -H 'Content-Type: application/json' -d '{\"restaurant_id\":\"r1\",\"items\":[{\"menu_item_id\":\"m1\",\"quantity\":2}]}'"
printf 'Logs: %s compose logs -f order\n' "${DOCKER[*]}"
printf 'Stop: %s compose down\n' "${DOCKER[*]}"
