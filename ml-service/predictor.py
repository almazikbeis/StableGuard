"""Depeg predictor using Amazon Chronos T5 Small."""
import numpy as np
import torch
from chronos import ChronosPipeline


PEG_TARGET = 1.0
WARNING_THRESHOLD = 0.998   # 0.2% depeg
DANGER_THRESHOLD  = 0.995   # 0.5% depeg — circuit-breaker zone


class DepegPredictor:
    def __init__(self):
        print("[predictor] Loading amazon/chronos-t5-small …")
        self.pipeline = ChronosPipeline.from_pretrained(
            "amazon/chronos-t5-small",
            device_map="cpu",
            torch_dtype=torch.float32,
        )
        print("[predictor] Model ready.")

    def predict(self, prices: list[float], steps: int = 20) -> dict:
        """
        Given a list of historical prices, forecast the next `steps` points.
        Returns median + 10th/90th percentile confidence intervals + depeg probability.
        """
        if len(prices) < 5:
            raise ValueError("Need at least 5 price points")

        context = torch.tensor(prices, dtype=torch.float32).unsqueeze(0)

        # 200 Monte-Carlo samples for robust confidence intervals
        forecast = self.pipeline.predict(
            context,
            prediction_length=steps,
            num_samples=200,
        )
        samples = forecast[0].numpy()  # shape: (200, steps)

        low    = np.percentile(samples, 10, axis=0)
        median = np.percentile(samples, 50, axis=0)
        high   = np.percentile(samples, 90, axis=0)

        # Clamp — stablecoin prices shouldn't be negative or > 1.05
        low    = np.clip(low,    0.95, 1.05)
        median = np.clip(median, 0.95, 1.05)
        high   = np.clip(high,   0.95, 1.05)

        # Probability: fraction of samples that touch warning / danger zone
        depeg_prob  = float(np.mean(np.any(samples < WARNING_THRESHOLD, axis=1)))
        severe_prob = float(np.mean(np.any(samples < DANGER_THRESHOLD,  axis=1)))

        # Trend direction from last historical point to median forecast end
        last_hist = prices[-1]
        first_pred = float(median[0])
        last_pred  = float(median[-1])

        if last_pred < last_hist - 0.0005:
            trend = "declining"
        elif last_pred > last_hist + 0.0005:
            trend = "recovering"
        else:
            trend = "stable"

        # Hours until predicted price first crosses warning threshold (if ever)
        hours_to_warning: float | None = None
        step_hours = (steps / 20) * 4 / steps  # 20 steps = 4 h by default
        for i, p in enumerate(median):
            if p < WARNING_THRESHOLD:
                hours_to_warning = round((i + 1) * step_hours, 1)
                break

        return {
            "predictions":        median.tolist(),
            "low":                low.tolist(),
            "high":               high.tolist(),
            "depeg_probability":  round(depeg_prob  * 100, 1),
            "severe_probability": round(severe_prob * 100, 1),
            "trend":              trend,
            "horizon_steps":      steps,
            "step_minutes":       int(round(4 * 60 / steps)),  # 12 min per step
            "min_predicted":      float(np.min(median)),
            "max_predicted":      float(np.max(median)),
            "hours_to_warning":   hours_to_warning,
        }
