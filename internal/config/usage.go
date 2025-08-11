package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Global usage persistence under $HOME/.loom/usages/aggregates.json

type GlobalUsageTotals struct {
	TotalInTokens  int64                          `json:"total_in_tokens"`
	TotalOutTokens int64                          `json:"total_out_tokens"`
	TotalInUSD     float64                        `json:"total_in_usd"`
	TotalOutUSD    float64                        `json:"total_out_usd"`
	PerProvider    map[string]GlobalProviderUsage `json:"per_provider"`
	PerModel       map[string]GlobalModelUsage    `json:"per_model"`
}

type GlobalProviderUsage struct {
	InTokens    int64   `json:"in_tokens"`
	OutTokens   int64   `json:"out_tokens"`
	TotalTokens int64   `json:"total_tokens"`
	InUSD       float64 `json:"in_usd"`
	OutUSD      float64 `json:"out_usd"`
	TotalUSD    float64 `json:"total_usd"`
}

type GlobalModelUsage struct {
	Provider    string  `json:"provider"`
	InTokens    int64   `json:"in_tokens"`
	OutTokens   int64   `json:"out_tokens"`
	TotalTokens int64   `json:"total_tokens"`
	InUSD       float64 `json:"in_usd"`
	OutUSD      float64 `json:"out_usd"`
	TotalUSD    float64 `json:"total_usd"`
}

var globalUsageMu sync.Mutex

func globalUsagePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HOME: %w", err)
	}
	dir := filepath.Join(home, ".loom", "usages")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create usages directory: %w", err)
	}
	return filepath.Join(dir, "aggregates.json"), nil
}

func loadGlobalUsage() (GlobalUsageTotals, error) {
	var totals GlobalUsageTotals
	path, err := globalUsagePath()
	if err != nil {
		return totals, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// initialize empty
			totals.PerProvider = make(map[string]GlobalProviderUsage)
			totals.PerModel = make(map[string]GlobalModelUsage)
			return totals, nil
		}
		return totals, err
	}
	if len(data) == 0 {
		totals.PerProvider = make(map[string]GlobalProviderUsage)
		totals.PerModel = make(map[string]GlobalModelUsage)
		return totals, nil
	}
	if err := json.Unmarshal(data, &totals); err != nil {
		// If corrupt, start fresh
		totals.PerProvider = make(map[string]GlobalProviderUsage)
		totals.PerModel = make(map[string]GlobalModelUsage)
		return totals, nil
	}
	if totals.PerProvider == nil {
		totals.PerProvider = make(map[string]GlobalProviderUsage)
	}
	if totals.PerModel == nil {
		totals.PerModel = make(map[string]GlobalModelUsage)
	}
	return totals, nil
}

func saveGlobalUsage(totals GlobalUsageTotals) error {
	path, err := globalUsagePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(totals, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// AddGlobalUsage increments the global usage aggregate across all projects.
func AddGlobalUsage(provider string, model string, inTokens, outTokens int64, inUSD, outUSD float64) error {
	globalUsageMu.Lock()
	defer globalUsageMu.Unlock()

	totals, _ := loadGlobalUsage()
	totals.TotalInTokens += inTokens
	totals.TotalOutTokens += outTokens
	totals.TotalInUSD += inUSD
	totals.TotalOutUSD += outUSD

	pp := totals.PerProvider[provider]
	pp.InTokens += inTokens
	pp.OutTokens += outTokens
	pp.TotalTokens += inTokens + outTokens
	pp.InUSD += inUSD
	pp.OutUSD += outUSD
	pp.TotalUSD += inUSD + outUSD
	totals.PerProvider[provider] = pp

	pm := totals.PerModel[model]
	pm.Provider = provider
	pm.InTokens += inTokens
	pm.OutTokens += outTokens
	pm.TotalTokens += inTokens + outTokens
	pm.InUSD += inUSD
	pm.OutUSD += outUSD
	pm.TotalUSD += inUSD + outUSD
	totals.PerModel[model] = pm

	return saveGlobalUsage(totals)
}

// GetGlobalUsage returns the global usage totals.
func GetGlobalUsage() GlobalUsageTotals {
	totals, _ := loadGlobalUsage()
	return totals
}

// ResetGlobalUsage clears the global usage aggregates.
func ResetGlobalUsage() error {
	globalUsageMu.Lock()
	defer globalUsageMu.Unlock()
	path, err := globalUsagePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
