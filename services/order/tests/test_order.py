import importlib.util
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import httpx
import pytest
from fastapi.testclient import TestClient

spec = importlib.util.spec_from_file_location("order_app", Path(__file__).resolve().parents[1] / "app.py")
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
app = mod.app

VALID_RESTAURANT = {
    "id": "r1",
    "name": "Dhaka Kitchen",
    "items": [
        {"id": "m1", "name": "Biryani", "available": True},
        {"id": "m2", "name": "Kebab", "available": True},
    ],
}
VALID_ORDER = {"restaurant_id": "r1", "items": [{"menu_item_id": "m1", "quantity": 2}]}


class _Resp:
    def __init__(self, status_code, body):
        self.status_code = status_code
        self._body = body

    def json(self):
        return self._body


def mock_restaurant(monkeypatch, status_code=200, body=None, exc=None):
    monkeypatch.setenv("RESTAURANT_URL", "http://restaurant")

    class Client:
        def __init__(self, *a, **k):
            pass

        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def get(self, url, headers=None):
            if exc:
                raise exc
            return _Resp(status_code, VALID_RESTAURANT if body is None else body)

    monkeypatch.setattr(httpx, "AsyncClient", Client)


@pytest.fixture(scope="module")
def order_client():
    with TestClient(app) as client:
        yield client


def test_healthz(order_client):
    r = order_client.get("/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_create_and_get_order(order_client, monkeypatch):
    mock_restaurant(monkeypatch)
    r = order_client.post("/orders", json=VALID_ORDER, headers={"X-Correlation-ID": "cid-1"})
    assert r.status_code == 201
    order = r.json()
    assert order["order_id"] and order["status"] == "created" and order["created_at"]
    assert order["correlation_id"] == "cid-1"
    got = order_client.get(f"/orders/{order['order_id']}")
    assert got.status_code == 200 and got.json() == order


def test_malformed_json(order_client):
    r = order_client.post("/orders", content="{", headers={"Content-Type": "application/json"})
    assert r.status_code == 400 and r.json() == {"error": "malformed_json"}


def test_missing_fields(order_client):
    r = order_client.post("/orders", json={"restaurant_id": "r1"})
    assert r.status_code == 400 and r.json() == {"error": "missing_fields"}


def test_invalid_quantity(order_client):
    r = order_client.post("/orders", json={"restaurant_id": "r1", "items": [{"menu_item_id": "m1", "quantity": 0}]})
    assert r.status_code == 400 and r.json() == {"error": "invalid_item"}


def test_duplicate_item(order_client):
    body = {"restaurant_id": "r1", "items": [{"menu_item_id": "m1", "quantity": 1}, {"menu_item_id": "m1", "quantity": 2}]}
    r = order_client.post("/orders", json=body)
    assert r.status_code == 400 and r.json() == {"error": "duplicate_item"}


def test_restaurant_not_found(order_client, monkeypatch):
    mock_restaurant(monkeypatch, status_code=404, body={"error": "restaurant_not_found"})
    r = order_client.post("/orders", json=VALID_ORDER)
    assert r.status_code == 404 and r.json() == {"error": "restaurant_not_found"}


def test_menu_item_not_found(order_client, monkeypatch):
    mock_restaurant(monkeypatch)
    r = order_client.post("/orders", json={"restaurant_id": "r1", "items": [{"menu_item_id": "missing", "quantity": 1}]})
    assert r.status_code == 404 and r.json() == {"error": "menu_item_not_found"}


def test_dependency_unavailable(order_client, monkeypatch):
    mock_restaurant(monkeypatch, exc=httpx.ReadTimeout("timeout"))
    start = time.monotonic()
    r = order_client.post("/orders", json=VALID_ORDER)
    assert r.status_code == 503 and r.json() == {"error": "dependency_unavailable"}
    assert time.monotonic() - start < 1


def test_get_order_not_found(order_client):
    r = order_client.get("/orders/missing")
    assert r.status_code == 404 and r.json() == {"error": "order_not_found"}


def test_concurrent_orders_unique(order_client, monkeypatch):
    mock_restaurant(monkeypatch)
    with ThreadPoolExecutor(max_workers=20) as pool:
        responses = list(pool.map(lambda _: order_client.post("/orders", json=VALID_ORDER), range(20)))
    assert all(r.status_code == 201 for r in responses)
    assert len({r.json()["order_id"] for r in responses}) == 20

