#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${ORDER_URL:-http://localhost:8082}"
COUNT="${COUNT:-21}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "== Concurrent Order Report =="
echo "target=$BASE_URL/orders requests=$COUNT"

for i in $(seq 1 "$COUNT"); do
  curl -sS -o "$tmp/$i.body" -w "%{http_code}" -X POST "$BASE_URL/orders" \
    -H 'Content-Type: application/json' \
    -H "X-Correlation-ID: conc-$i" \
    -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}' > "$tmp/$i.code" &
done
wait

ok=0
echo
echo "Created orders:"
for i in $(seq 1 "$COUNT"); do
  code=$(cat "$tmp/$i.code")
  id=$(grep -ho '"order_id":"[^"]*"' "$tmp/$i.body" | head -n1 | cut -d'"' -f4 || true)
  if [ "$code" = "201" ] && [ -n "$id" ]; then
    ok=$((ok + 1))
    echo "  - order_id=$id url=$BASE_URL/orders/$id"
  else
    echo "  - request=$i status=$code body=$(cat "$tmp/$i.body")"
  fi
done

ids=$(for i in $(seq 1 "$COUNT"); do grep -ho '"order_id":"[^"]*"' "$tmp/$i.body" | head -n1 | cut -d'"' -f4 || true; done | sed '/^$/d' | sort -u)
unique=$(printf '%s\n' "$ids" | sed '/^$/d' | wc -l | tr -d ' ')

echo
echo "summary: success=$ok/$COUNT unique_ids=$unique"
if [ "$ok" -lt 20 ] || [ "$unique" -lt 20 ]; then
  echo "RESULT: FAIL (need >=20 successful orders with >=20 unique IDs)"
  exit 1
fi

echo "RESULT: PASS (>=20 orders created successfully with unique IDs)"
