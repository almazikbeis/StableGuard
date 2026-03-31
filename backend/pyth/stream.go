// Package pyth — real-time price streaming via Hermes SSE.
// Falls back to polling if SSE is unavailable.
package pyth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Streamer connects to Pyth Hermes Server-Sent Events and pushes PriceSnapshots.
// Automatically reconnects on disconnection and falls back to polling on repeated SSE failure.
type Streamer struct {
	monitor      *Monitor
	hermesURL    string
	client       *http.Client
	pollInterval time.Duration
}

// NewStreamer creates a streamer. pollInterval is used when SSE is unavailable.
func NewStreamer(hermesURL string, pollInterval time.Duration) *Streamer {
	return &Streamer{
		monitor:      New(hermesURL),
		hermesURL:    hermesURL,
		client:       &http.Client{Timeout: 0}, // streaming — no deadline
		pollInterval: pollInterval,
	}
}

// Start streams price updates into out until ctx is cancelled.
// Tries SSE first; if it fails 3 times in a row, switches to polling mode.
func (s *Streamer) Start(ctx context.Context, out chan<- *PriceSnapshot) {
	sseFailures := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if sseFailures >= 3 {
			log.Printf("[pyth/stream] SSE unavailable, using polling fallback (%v)", s.pollInterval)
			s.runPollLoop(ctx, out)
			return
		}

		if err := s.connectSSE(ctx, out); err != nil {
			sseFailures++
			log.Printf("[pyth/stream] SSE error (%d/3): %v — retry in 5s", sseFailures, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		} else {
			// clean disconnect (ctx done)
			return
		}
	}
}

// connectSSE opens one SSE connection and reads events until error or ctx cancel.
func (s *Streamer) connectSSE(ctx context.Context, out chan<- *PriceSnapshot) error {
	// Build URL with all active feed IDs
	ids := AllFeedIDs()
	params := make([]string, len(ids))
	for i, id := range ids {
		params[i] = "ids[]=" + id
	}
	url := fmt.Sprintf("%s/v2/updates/price/stream?%s&parsed=true",
		s.hermesURL, strings.Join(params, "&"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hermes SSE returned HTTP %d", resp.StatusCode)
	}

	log.Printf("[pyth/stream] SSE connected — monitoring %d feeds", len(ids))
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		snap := parseSSEPayload(payload)
		if snap == nil {
			continue
		}

		select {
		case out <- snap:
			log.Printf("[pyth/stream] update usdc=%.6f usdt=%.6f dev=%.5f%% tokens=%d",
				snap.USDC.Price, snap.USDT.Price, snap.Deviation(), len(snap.All))
		case <-ctx.Done():
			return nil
		default:
			// drop — consumer is busy
		}
	}

	return scanner.Err()
}

// runPollLoop polls Monitor at pollInterval — fallback when SSE is down.
func (s *Streamer) runPollLoop(ctx context.Context, out chan<- *PriceSnapshot) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := s.monitor.FetchSnapshot()
			if err != nil {
				log.Printf("[pyth/stream] poll error: %v", err)
				continue
			}
			select {
			case out <- snap:
				log.Printf("[pyth/stream] poll usdc=%.6f usdt=%.6f dev=%.5f%% tokens=%d",
					snap.USDC.Price, snap.USDT.Price, snap.Deviation(), len(snap.All))
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// parseSSEPayload parses a single SSE data line into a PriceSnapshot.
func parseSSEPayload(payload string) *PriceSnapshot {
	var hr hermesResponse
	if err := json.Unmarshal([]byte(payload), &hr); err != nil {
		return nil
	}

	snap := buildSnapshot(&hr)

	// Require at least USDC or USDT to be present for a valid snapshot
	if snap.USDC.Price == 0 && snap.USDT.Price == 0 {
		return nil
	}
	return snap
}
