#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${ORDER_URL:-http://localhost:8082}"
N="${N:-20}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for i in $(seq 1 "$N"); do
  curl -sS -o "$tmp/$i.body" -w "%{http_code}" -X POST "$BASE_URL/orders" \
    -H 'Content-Type: application/json' \
    -d '{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}' > "$tmp/$i.code" &
done
wait

ok=0
ids=()
for i in $(seq 1 "$N"); do
  [ "$(cat "$tmp/$i.code")" = "201" ] || { echo "fail[$i]: status=$(cat "$tmp/$i.code") body=$(cat "$tmp/$i.body")"; continue; }
  id=$(grep -ho '"order_id":"[^"]*"' "$tmp/$i.body" | head -n1 | cut -d'"' -f4)
  [ -n "$id" ] || { echo "fail[$i]: missing order_id"; continue; }
  ok=$((ok + 1))
  ids+=("$id")
  echo "ok[$i]: order_id=$id"
done

unique=$(printf '%s\n' "${ids[@]:-}" | sed '/^$/d' | sort -u | wc -l | tr -d ' ')
echo "summary: success=$ok/$N unique_ids=$unique"
[ "$ok" -eq "$N" ] && [ "$unique" -eq "$N" ] && echo "PASS" || { echo "FAIL"; exit 1; }
