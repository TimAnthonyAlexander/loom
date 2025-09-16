package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Price represents USD cost per token for input and output.
// Values are in USD per single token.
type Price struct {
	InPerToken  float64
	OutPerToken float64
}

// OpenRouterModel represents a model from the OpenRouter API
type OpenRouterModel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Pricing *struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

// OpenRouterResponse represents the API response structure
type OpenRouterResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// OpenRouter pricing cache
var (
	openRouterPrices     = make(map[string]Price)
	openRouterCacheTime  time.Time
	openRouterCacheMutex sync.RWMutex
	openRouterCacheTTL   = 1 * time.Hour
)

// PricePerToken holds model identifier to price mapping.
// Keep this list refreshed from provider docs periodically.
var PricePerToken = map[string]Price{
	// Anthropic
	"claude-opus-4-20250514":   {InPerToken: 15.0 / 1e6, OutPerToken: 75.0 / 1e6},
	"claude-sonnet-4-20250514": {InPerToken: 3.0 / 1e6, OutPerToken: 15.0 / 1e6},
	// Treat Haiku 4 as Haiku 3.5 until official
	"claude-haiku-4-20250514":    {InPerToken: 0.80 / 1e6, OutPerToken: 4.0 / 1e6},
	"claude-3-7-sonnet-20250219": {InPerToken: 3.0 / 1e6, OutPerToken: 15.0 / 1e6},
	"claude-3-5-sonnet-20241022": {InPerToken: 3.0 / 1e6, OutPerToken: 15.0 / 1e6},
	"claude-3-5-haiku-20241022":  {InPerToken: 0.80 / 1e6, OutPerToken: 4.0 / 1e6},
	"claude-3-opus-20240229":     {InPerToken: 15.0 / 1e6, OutPerToken: 75.0 / 1e6},
	"claude-3-sonnet-20240229":   {InPerToken: 3.0 / 1e6, OutPerToken: 15.0 / 1e6},
	"claude-3-haiku-20240307":    {InPerToken: 0.25 / 1e6, OutPerToken: 1.25 / 1e6},

	// OpenAI
	"gpt-5":        {InPerToken: 1.25 / 1e6, OutPerToken: 10.0 / 1e6},
	"gpt-5-codex":  {InPerToken: 1.25 / 1e6, OutPerToken: 10.0 / 1e6},
	"gpt-4.1":      {InPerToken: 2.0 / 1e6, OutPerToken: 8.0 / 1e6},
	"o4-mini":      {InPerToken: 1.10 / 1e6, OutPerToken: 4.40 / 1e6},
	"o3":           {InPerToken: 2.0 / 1e6, OutPerToken: 8.0 / 1e6},
	"o3-mini":      {InPerToken: 1.10 / 1e6, OutPerToken: 4.40 / 1e6},
	"gpt-4.1-mini": {InPerToken: 0.40 / 1e6, OutPerToken: 1.60 / 1e6},
	"gpt-4.1-nano": {InPerToken: 0.10 / 1e6, OutPerToken: 0.40 / 1e6},
}

// fetchOpenRouterPrices fetches model pricing from OpenRouter API and caches it
func fetchOpenRouterPrices() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenRouter API returned status %d", resp.StatusCode)
	}

	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	openRouterCacheMutex.Lock()
	defer openRouterCacheMutex.Unlock()

	openRouterPrices = make(map[string]Price)

	for _, model := range apiResp.Data {
		if model.ID == "" || model.Pricing == nil {
			continue
		}
		promptPrice, err1 := strconv.ParseFloat(model.Pricing.Prompt, 64)
		completionPrice, err2 := strconv.ParseFloat(model.Pricing.Completion, 64)
		if err1 == nil && err2 == nil {
			openRouterPrices[model.ID] = Price{
				InPerToken:  promptPrice,
				OutPerToken: completionPrice,
			}
		}
	}

	openRouterCacheTime = time.Now()
	return nil
}

// getOpenRouterPrice returns cached pricing for an OpenRouter model, fetching if needed
func getOpenRouterPrice(modelID string) (Price, bool) {
	openRouterCacheMutex.RLock()

	// Check if cache is still valid
	if time.Since(openRouterCacheTime) > openRouterCacheTTL {
		openRouterCacheMutex.RUnlock()

		// Cache expired, try to refresh (but don't block if it fails)
		go func() {
			_ = fetchOpenRouterPrices()
		}()

		// Use stale cache if available
		openRouterCacheMutex.RLock()
		price, ok := openRouterPrices[modelID]
		openRouterCacheMutex.RUnlock()
		return price, ok
	}

	price, ok := openRouterPrices[modelID]
	openRouterCacheMutex.RUnlock()
	return price, ok
}

// CostUSDParts computes separate input and output USD costs and their sum for a given model and usage.
// Provider string is accepted for UI purposes but currently not used for pricing lookup.
func CostUSDParts(model string, inTokens, outTokens int64) (inUSD, outUSD, totalUSD float64) {
	// First check static pricing table
	if p, ok := PricePerToken[model]; ok {
		inUSD = float64(inTokens) * p.InPerToken
		outUSD = float64(outTokens) * p.OutPerToken
		return inUSD, outUSD, inUSD + outUSD
	}

	// For unknown models, try OpenRouter dynamic pricing
	if p, ok := getOpenRouterPrice(model); ok {
		inUSD = float64(inTokens) * p.InPerToken
		outUSD = float64(outTokens) * p.OutPerToken
		return inUSD, outUSD, inUSD + outUSD
	}

	// If cache is empty, try to fetch once synchronously
	openRouterCacheMutex.RLock()
	isEmpty := len(openRouterPrices) == 0
	openRouterCacheMutex.RUnlock()

	if isEmpty {
		if err := fetchOpenRouterPrices(); err == nil {
			if p, ok := getOpenRouterPrice(model); ok {
				inUSD = float64(inTokens) * p.InPerToken
				outUSD = float64(outTokens) * p.OutPerToken
				return inUSD, outUSD, inUSD + outUSD
			}
		}
	}

	// Unknown model: default to zero pricing to avoid surprises
	return 0, 0, 0
}
