// Package prediction is a Go client for the Python Chronos ML microservice.
package prediction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

var mlServiceURL = func() string {
	if u := os.Getenv("ML_SERVICE_URL"); u != "" {
		return u
	}
	return "http://localhost:8001"
}()

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Result is the structured response from the Python service.
type Result struct {
	Symbol            string    `json:"symbol"`
	Predictions       []float64 `json:"predictions"`
	Low               []float64 `json:"low"`
	High              []float64 `json:"high"`
	DepegProbability  float64   `json:"depeg_probability"`
	SevereProbability float64   `json:"severe_probability"`
	Trend             string    `json:"trend"`
	HorizonSteps      int       `json:"horizon_steps"`
	StepMinutes       int       `json:"step_minutes"`
	MinPredicted      float64   `json:"min_predicted"`
	MaxPredicted      float64   `json:"max_predicted"`
	HoursToWarning    *float64  `json:"hours_to_warning"`
	InferenceMs       int       `json:"inference_ms"`
}

// ── Cache ─────────────────────────────────────────────────────────────────

type cacheEntry struct {
	result    *Result
	fetchedAt time.Time
}

var (
	cacheMu  sync.RWMutex
	cache    = map[string]cacheEntry{}
	cacheTTL = 5 * time.Minute
)

// Predict fetches a depeg forecast for the given symbol + price history.
// Results are cached for 5 minutes — safe to call on every pipeline tick.
func Predict(symbol string, prices []float64) (*Result, error) {
	key := symbol

	cacheMu.RLock()
	if e, ok := cache[key]; ok && time.Since(e.fetchedAt) < cacheTTL {
		cacheMu.RUnlock()
		return e.result, nil
	}
	cacheMu.RUnlock()

	result, err := callService(symbol, prices)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	cache[key] = cacheEntry{result: result, fetchedAt: time.Now()}
	cacheMu.Unlock()

	return result, nil
}

// Healthy pings the ML service health endpoint.
func Healthy() bool {
	resp, err := httpClient.Get(mlServiceURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func callService(symbol string, prices []float64) (*Result, error) {
	req := map[string]any{
		"symbol": symbol,
		"prices": prices,
		"steps":  20, // 20 steps × 12 min = 4 hours
	}
	body, _ := json.Marshal(req)

	resp, err := httpClient.Post(mlServiceURL+"/predict", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ml service unavailable: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml service %d: %s", resp.StatusCode, string(data))
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal prediction: %w", err)
	}
	return &result, nil
}
