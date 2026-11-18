from fastapi.testclient import TestClient

from app import app

client = TestClient(app)


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
    assert sum(len(restaurant["items"]) for restaurant in restaurants) >= 6


def test_get_restaurant():
    response = client.get("/restaurants/r1")
    assert response.status_code == 200
    assert response.json()["id"] == "r1"

    missing = client.get("/restaurants/missing")
    assert missing.status_code == 404
    assert missing.json() == {"error": "restaurant_not_found"}
