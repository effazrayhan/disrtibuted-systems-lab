#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

DOCKER=(docker)
if ! docker info >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
fi

"${DOCKER[@]}" compose up -d --build

for _ in $(seq 1 30); do
  if curl -fsS http://localhost:8081/healthz >/dev/null 2>&1 && curl -fsS http://localhost:8082/healthz >/dev/null 2>&1; then
    echo "Services ready: restaurant=http://localhost:8081 order=http://localhost:8082"
    exit 0
  fi
  sleep 1
done

"${DOCKER[@]}" compose logs >&2
echo "Services not ready" >&2
exit 1
