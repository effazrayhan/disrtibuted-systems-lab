import importlib.util
from pathlib import Path

from fastapi.testclient import TestClient

spec = importlib.util.spec_from_file_location("restaurant_app", Path(__file__).resolve().parents[1] / "app.py")
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
client = TestClient(mod.app)


def test_healthz():
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    assert response.headers["X-Correlation-ID"]


def test_list_restaurants():
    response = client.get("/restaurants")
    assert response.status_code == 200
    restaurants = response.json()["restaurants"]
    assert len(restaurants) >= 3
    assert sum(len(r["items"]) for r in restaurants) >= 6


def test_get_restaurant():
    response = client.get("/restaurants/r1")
    assert response.status_code == 200
    assert response.json()["id"] == "r1"

    missing = client.get("/restaurants/missing")
    assert missing.status_code == 404
    assert missing.json() == {"error": "restaurant_not_found"}
