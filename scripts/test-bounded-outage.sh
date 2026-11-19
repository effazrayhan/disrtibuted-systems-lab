#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${ORDER_URL:-http://localhost:8082}"
THRESHOLD_MS="${THRESHOLD_MS:-10000}"

echo "== Bounded Outage Report =="
echo "target=$BASE_URL/orders threshold_ms=$THRESHOLD_MS"
echo "precondition: Restaurant service must be unavailable (e.g. docker compose stop restaurant)"
echo

start_ms=$(date +%s%3N)
body_file=$(mktemp)
http_code=$(curl -sS -o "$body_file" -w "%{http_code}" -X POST "$BASE_URL/orders" \
  -H 'Content-Type: application/json' \
  -H 'X-Correlation-ID: outage-test' \
  -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}')
end_ms=$(date +%s%3N)
elapsed=$((end_ms - start_ms))
body=$(cat "$body_file")
rm -f "$body_file"

echo "response: status=$http_code elapsed_ms=$elapsed body=$body"
echo "test_link: curl -i -X POST $BASE_URL/orders -H 'Content-Type: application/json' -d '{\"restaurant_id\":\"r1\",\"items\":[{\"menu_item_id\":\"m1\",\"quantity\":1}]}'"
echo

if [ "$http_code" = "503" ] && printf '%s' "$body" | grep -q 'dependency_unavailable' && [ "$elapsed" -lt "$THRESHOLD_MS" ]; then
  echo "RESULT: PASS (bounded 503 dependency_unavailable)"
  exit 0
fi

echo "RESULT: FAIL (expected 503 dependency_unavailable within ${THRESHOLD_MS}ms)"
exit 1
