#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "== Starting Docker =="
if ! docker info >/dev/null 2>&1 && ! sudo docker info >/dev/null 2>&1; then
  if command -v systemctl >/dev/null 2>&1; then
    sudo systemctl start docker || true
  elif command -v service >/dev/null 2>&1; then
    sudo service docker start || true
  fi
fi

for _ in $(seq 1 30); do
  if docker info >/dev/null 2>&1 || sudo docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if docker info >/dev/null 2>&1; then
  docker --version
  echo "Docker daemon: running"
elif sudo docker info >/dev/null 2>&1; then
  sudo docker --version
  echo "Docker daemon: running (via sudo)"
else
  echo "Docker daemon: not available (continuing with pytest)"
fi

echo
echo "== Preparing Python environment =="
if [ ! -d .venv ]; then
  python3 -m venv .venv
fi
# shellcheck disable=SC1091
source .venv/bin/activate
pip install -q -r services/restaurant/requirements.txt -r services/order/requirements.txt -r requirements-dev.txt

set +e
echo
echo "== Restaurant service tests =="
(cd services/restaurant && pytest -q)
r1=$?

echo
echo "== Order service tests =="
(cd services/order && pytest -q)
r2=$?
set -e

echo
echo "== Summary =="
echo "restaurant_exit=$r1 order_exit=$r2"
if [ "$r1" -eq 0 ] && [ "$r2" -eq 0 ]; then
  echo "RESULT: PASS"
  exit 0
fi

echo "RESULT: FAIL"
exit 1
