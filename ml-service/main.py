"""StableGuard ML Service — Chronos T5 depeg forecasting.

Run:
    pip install -r requirements.txt
    python main.py

Or with uvicorn directly:
    uvicorn main:app --host 0.0.0.0 --port 8001
"""
import time
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from predictor import DepegPredictor

# ── Globals ────────────────────────────────────────────────────────────────
predictor: Optional[DepegPredictor] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global predictor
    predictor = DepegPredictor()
    yield
    # nothing to clean up


app = FastAPI(
    title="StableGuard ML Service",
    description="Chronos T5 depeg forecasting microservice",
    version="1.0.0",
    lifespan=lifespan,
)


# ── Schemas ────────────────────────────────────────────────────────────────

class PredictRequest(BaseModel):
    symbol: str = "USDC"
    prices: list[float]
    steps:  int = 20


class PredictResponse(BaseModel):
    symbol:            str
    predictions:       list[float]
    low:               list[float]
    high:              list[float]
    depeg_probability: float   # 0–100 %
    severe_probability:float   # 0–100 %
    trend:             str     # "stable" | "declining" | "recovering"
    horizon_steps:     int
    step_minutes:      int
    min_predicted:     float
    max_predicted:     float
    hours_to_warning:  float | None
    inference_ms:      int


# ── Routes ─────────────────────────────────────────────────────────────────

@app.get("/health")
def health():
    return {"ok": True, "model": "amazon/chronos-t5-small", "ready": predictor is not None}


@app.post("/predict", response_model=PredictResponse)
def predict(req: PredictRequest):
    if predictor is None:
        raise HTTPException(503, "Model not loaded yet")
    if len(req.prices) < 5:
        raise HTTPException(400, "Need at least 5 price points")
    if req.steps < 1 or req.steps > 64:
        raise HTTPException(400, "steps must be 1–64")

    t0 = time.monotonic()
    result = predictor.predict(req.prices, req.steps)
    elapsed_ms = int((time.monotonic() - t0) * 1000)

    return PredictResponse(
        symbol=req.symbol,
        inference_ms=elapsed_ms,
        **result,
    )


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8001, reload=False)
