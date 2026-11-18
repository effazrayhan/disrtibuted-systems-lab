import logging
import os
import time
import uuid

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
logger = logging.getLogger("restaurant")

SEED_RESTAURANTS = [
    {
        "id": "r1",
        "name": "Dhaka Kitchen",
        "items": [
            {"id": "m1", "name": "Chicken Biryani", "price_cents": 35000, "available": True},
            {"id": "m2", "name": "Seekh Kebab", "price_cents": 28000, "available": True},
            {"id": "m3", "name": "Mango Lassi", "price_cents": 12000, "available": True},
        ],
    },
    {
        "id": "r2",
        "name": "Chattogram Cafe",
        "items": [
            {"id": "m4", "name": "Beef Kala Bhuna", "price_cents": 42000, "available": True},
            {"id": "m5", "name": "Paratha", "price_cents": 9000, "available": True},
            {"id": "m6", "name": "Faluda", "price_cents": 15000, "available": True},
        ],
    },
    {
        "id": "r3",
        "name": "Sylhet Grill",
        "items": [
            {"id": "m7", "name": "Grilled Chicken", "price_cents": 39000, "available": True},
            {"id": "m8", "name": "Garlic Naan", "price_cents": 8000, "available": True},
            {"id": "m9", "name": "Lime Soda", "price_cents": 7000, "available": True},
        ],
    },
]

RESTAURANTS = {restaurant["id"]: restaurant for restaurant in SEED_RESTAURANTS}

app = FastAPI(title="Restaurant Service")


@app.middleware("http")
async def correlation_middleware(request: Request, call_next):
    correlation_id = request.headers.get("X-Correlation-ID") or uuid.uuid4().hex
    request.state.correlation_id = correlation_id
    start = time.monotonic()
    response = await call_next(request)
    response.headers["X-Correlation-ID"] = correlation_id
    elapsed_ms = int((time.monotonic() - start) * 1000)
    logger.info(
        "cid=%s method=%s path=%s duration_ms=%d",
        correlation_id,
        request.method,
        request.url.path,
        elapsed_ms,
    )
    return response


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.get("/restaurants")
def list_restaurants():
    return {"restaurants": SEED_RESTAURANTS}


@app.get("/restaurants/{restaurant_id}")
def get_restaurant(restaurant_id: str):
    restaurant = RESTAURANTS.get(restaurant_id)
    if restaurant is None:
        return JSONResponse(status_code=404, content={"error": "restaurant_not_found"})
    return restaurant


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app:app", host="0.0.0.0", port=int(os.getenv("PORT", "8081")), log_level="info")
