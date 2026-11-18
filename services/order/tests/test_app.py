import json
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import httpx
import pytest
import uvicorn

from app import app

VALID_RESTAURANT = {
    "id": "r1",
    "name": "Dhaka Kitchen",
    "items": [
        {"id": "m1", "name": "Biryani", "available": True},
        {"id": "m2", "name": "Kebab", "available": True},
    ],
}

VALID_ORDER = {"restaurant_id": "r1", "items": [{"menu_item_id": "m1", "quantity": 2}]}


@pytest.fixture
def fake_restaurant():
    servers = []

    def create(status=200, body=None, delay=0):
        payload = json.dumps(body if body is not None else VALID_RESTAURANT).encode()

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if delay:
                    time.sleep(delay)
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)

            def log_message(self, *args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        servers.append((server, thread))
        return f"http://127.0.0.1:{server.server_port}"

    yield create

    for server, thread in servers:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


@pytest.fixture
def order_client():
    config = uvicorn.Config(app, host="127.0.0.1", port=0, log_level="error")
    server = uvicorn.Server(config)
    thread = threading.Thread(target=server.run, daemon=True)
    thread.start()
    while not server.started and thread.is_alive():
        time.sleep(0.05)
    port = server.servers[0].sockets[0].getsockname()[1]

    with httpx.Client(base_url=f"http://127.0.0.1:{port}", timeout=5) as client:
        yield client

    server.should_exit = True
    thread.join(timeout=5)


def test_healthz(order_client):
    response = order_client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_create_order_success(order_client, fake_restaurant, monkeypatch):
    monkeypatch.setenv("RESTAURANT_URL", fake_restaurant())
    response = order_client.post("/orders", json=VALID_ORDER, headers={"X-Correlation-ID": "cid-1"})
    assert response.status_code == 201

    order = response.json()
    assert order["order_id"]
    assert order["correlation_id"] == "cid-1"
    assert order["status"] == "created"
    assert order["created_at"].endswith("Z")

    fetched = order_client.get(f"/orders/{order['order_id']}")
    assert fetched.status_code == 200
    assert fetched.json() == order


def test_malformed_json(order_client):
    response = order_client.post("/orders", content="{", headers={"Content-Type": "application/json"})
    assert response.status_code == 400
    assert response.json() == {"error": "malformed_json"}


def test_missing_fields(order_client):
    response = order_client.post("/orders", json={"restaurant_id": "r1"})
    assert response.status_code == 400
    assert response.json() == {"error": "missing_fields"}


def test_invalid_quantity(order_client):
    body = {"restaurant_id": "r1", "items": [{"menu_item_id": "m1", "quantity": 0}]}
    response = order_client.post("/orders", json=body)
    assert response.status_code == 400
    assert response.json() == {"error": "invalid_item"}


def test_duplicate_item(order_client):
    body = {
        "restaurant_id": "r1",
        "items": [
            {"menu_item_id": "m1", "quantity": 1},
            {"menu_item_id": "m1", "quantity": 2},
        ],
    }
    response = order_client.post("/orders", json=body)
    assert response.status_code == 400
    assert response.json() == {"error": "duplicate_item"}


def test_restaurant_not_found(order_client, fake_restaurant, monkeypatch):
    monkeypatch.setenv(
        "RESTAURANT_URL",
        fake_restaurant(status=404, body={"error": "restaurant_not_found"}),
    )
    response = order_client.post("/orders", json=VALID_ORDER)
    assert response.status_code == 404
    assert response.json() == {"error": "restaurant_not_found"}


def test_menu_item_not_found(order_client, fake_restaurant, monkeypatch):
    monkeypatch.setenv("RESTAURANT_URL", fake_restaurant())
    body = {"restaurant_id": "r1", "items": [{"menu_item_id": "missing", "quantity": 1}]}
    response = order_client.post("/orders", json=body)
    assert response.status_code == 404
    assert response.json() == {"error": "menu_item_not_found"}


def test_dependency_unavailable(order_client, fake_restaurant, monkeypatch):
    monkeypatch.setenv("RESTAURANT_URL", fake_restaurant(delay=0.5))
    monkeypatch.setenv("RESTAURANT_TIMEOUT_MS", "100")
    start = time.monotonic()
    response = order_client.post("/orders", json=VALID_ORDER)
    elapsed = time.monotonic() - start
    assert response.status_code == 503
    assert response.json() == {"error": "dependency_unavailable"}
    assert elapsed < 2


def test_get_order_not_found(order_client):
    response = order_client.get("/orders/missing")
    assert response.status_code == 404
    assert response.json() == {"error": "order_not_found"}


def test_concurrent_orders_are_unique(order_client, fake_restaurant, monkeypatch):
    monkeypatch.setenv("RESTAURANT_URL", fake_restaurant())
    with ThreadPoolExecutor(max_workers=20) as pool:
        responses = list(pool.map(lambda _: order_client.post("/orders", json=VALID_ORDER), range(20)))

    assert all(response.status_code == 201 for response in responses)
    ids = [response.json()["order_id"] for response in responses]
    assert len(set(ids)) == 20
