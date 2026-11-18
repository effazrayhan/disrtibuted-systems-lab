import asyncio
import logging
import os
import time
import uuid
from datetime import datetime, timezone
from urllib.parse import quote

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
logger = logging.getLogger("order")

app = FastAPI(title="Order Service")

STORE: dict[str, dict] = {}
STORE_LOCK = asyncio.Lock()


def error_response(status_code: int, code: str) -> JSONResponse:
    return JSONResponse(status_code=status_code, content={"error": code})


def timeout_seconds() -> float:
    try:
        return max(float(os.getenv("RESTAURANT_TIMEOUT_MS", "2000")) / 1000, 0.001)
    except ValueError:
        return 2.0


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


@app.post("/orders")
async def create_order(request: Request):
    correlation_id = request.state.correlation_id

    try:
        body = await request.json()
    except (ValueError, UnicodeDecodeError):
        return error_response(400, "malformed_json")

    if not isinstance(body, dict):
        return error_response(400, "malformed_json")

    restaurant_id = body.get("restaurant_id")
    items = body.get("items")
    if not isinstance(restaurant_id, str) or not restaurant_id or not isinstance(items, list) or not items:
        return error_response(400, "missing_fields")

    normalized_items = []
    seen = set()
    for item in items:
        if not isinstance(item, dict):
            return error_response(400, "invalid_item")
        item_id = item.get("menu_item_id")
        quantity = item.get("quantity")
        if (
            not isinstance(item_id, str)
            or not item_id
            or not isinstance(quantity, int)
            or isinstance(quantity, bool)
            or quantity <= 0
        ):
            return error_response(400, "invalid_item")
        if item_id in seen:
            return error_response(400, "duplicate_item")
        seen.add(item_id)
        normalized_items.append({"menu_item_id": item_id, "quantity": quantity})

    base_url = os.getenv("RESTAURANT_URL", "").rstrip("/")
    if not base_url:
        logger.error("cid=%s event=missing_restaurant_url", correlation_id)
        return error_response(503, "dependency_unavailable")

    target = f"{base_url}/restaurants/{quote(restaurant_id, safe='')}"
    try:
        async with httpx.AsyncClient(timeout=timeout_seconds()) as client:
            response = await client.get(target, headers={"X-Correlation-ID": correlation_id})
    except httpx.HTTPError as exc:
        logger.warning("cid=%s event=dependency_error error=%s", correlation_id, exc)
        return error_response(503, "dependency_unavailable")

    if response.status_code == 404:
        return error_response(404, "restaurant_not_found")
    if response.status_code != 200:
        logger.warning("cid=%s event=dependency_status status=%d", correlation_id, response.status_code)
        return error_response(503, "dependency_unavailable")

    try:
        restaurant = response.json()
    except ValueError:
        logger.warning("cid=%s event=dependency_bad_json", correlation_id)
        return error_response(503, "dependency_unavailable")

    if not restaurant.get("id"):
        logger.warning("cid=%s event=dependency_bad_payload", correlation_id)
        return error_response(503, "dependency_unavailable")

    available = {item["id"] for item in restaurant.get("items", []) if item.get("available")}
    for item in normalized_items:
        if item["menu_item_id"] not in available:
            return error_response(404, "menu_item_not_found")

    created_at = datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")
    order = {
        "order_id": uuid.uuid4().hex,
        "correlation_id": correlation_id,
        "status": "created",
        "created_at": created_at,
        "restaurant_id": restaurant_id,
        "items": normalized_items,
    }
    async with STORE_LOCK:
        STORE[order["order_id"]] = order

    logger.info(
        "cid=%s order_id=%s restaurant_id=%s status=created",
        correlation_id,
        order["order_id"],
        restaurant_id,
    )
    return JSONResponse(status_code=201, content=order)


@app.get("/orders/{order_id}")
async def get_order(order_id: str):
    async with STORE_LOCK:
        order = STORE.get(order_id)
    if order is None:
        return error_response(404, "order_not_found")
    return order


if __name__ == "__main__":
    import uvicorn

    if not os.getenv("RESTAURANT_URL"):
        raise SystemExit("RESTAURANT_URL must be set")
    uvicorn.run("app:app", host="0.0.0.0", port=int(os.getenv("PORT", "8082")), log_level="info")
