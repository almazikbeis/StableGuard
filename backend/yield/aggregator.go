package yield

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// Aggregator fetches and caches yield opportunities from all adapters.
type Aggregator struct {
	adapters []Adapter

	mu        sync.RWMutex
	cached    []Opportunity
	cachedAt  time.Time
	cacheTTL  time.Duration
}

// NewAggregator creates an Aggregator with all supported adapters.
func NewAggregator() *Aggregator {
	return &Aggregator{
		adapters: []Adapter{
			NewKaminoAdapter(),
			NewMarginfiAdapter(),
			NewDriftAdapter(),
		},
		cacheTTL: 5 * time.Minute,
	}
}

// Opportunities returns all current opportunities, sorted by SupplyAPY descending.
// Results are cached for 5 minutes to avoid hammering external APIs.
func (a *Aggregator) Opportunities(ctx context.Context) []Opportunity {
	a.mu.RLock()
	if time.Since(a.cachedAt) < a.cacheTTL && len(a.cached) > 0 {
		cached := a.cached
		a.mu.RUnlock()
		return cached
	}
	a.mu.RUnlock()

	return a.refresh(ctx)
}

// BestFor returns the highest-APY opportunity for a given token (e.g. "USDC").
// Returns nil if no opportunity found.
func (a *Aggregator) BestFor(ctx context.Context, token string) *Opportunity {
	opps := a.Opportunities(ctx)
	for _, o := range opps { // already sorted highest first
		if o.Token == token {
			cp := o
			return &cp
		}
	}
	return nil
}

// refresh fetches from all adapters concurrently and updates the cache.
func (a *Aggregator) refresh(ctx context.Context) []Opportunity {
	type result struct {
		opps []Opportunity
	}
	results := make(chan result, len(a.adapters))

	fetchCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()

	for _, adapter := range a.adapters {
		go func(ad Adapter) {
			opps, err := ad.FetchOpportunities(fetchCtx)
			if err != nil {
				log.Printf("[yield/aggregator] adapter error: %v", err)
				opps = nil
			}
			results <- result{opps: opps}
		}(adapter)
	}

	var all []Opportunity
	for range a.adapters {
		r := <-results
		all = append(all, r.opps...)
	}

	sort.Sort(ByAPY(all))

	a.mu.Lock()
	a.cached = all
	a.cachedAt = time.Now()
	a.mu.Unlock()

	return all
}
