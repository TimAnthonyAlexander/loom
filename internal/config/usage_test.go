package config

import (
	"testing"
)

func floatEquals(a, b, eps float64) bool {
	if a > b {
		return a-b < eps
	}
	return b-a < eps
}

// Test global usage aggregation lifecycle: add → get → reset
func TestCostUSDParts_StaticPricing(t *testing.T) {
	// Test static pricing for known models
	inUSD, outUSD, totalUSD := CostUSDParts("claude-3-5-sonnet-20241022", 1000, 2000)

	expectedIn := float64(1000) * (3.0 / 1e6)   // $3 per 1M tokens
	expectedOut := float64(2000) * (15.0 / 1e6) // $15 per 1M tokens
	expectedTotal := expectedIn + expectedOut

	if !floatEquals(inUSD, expectedIn, 1e-9) {
		t.Errorf("input cost mismatch: got %f, want %f", inUSD, expectedIn)
	}
	if !floatEquals(outUSD, expectedOut, 1e-9) {
		t.Errorf("output cost mismatch: got %f, want %f", outUSD, expectedOut)
	}
	if !floatEquals(totalUSD, expectedTotal, 1e-9) {
		t.Errorf("total cost mismatch: got %f, want %f", totalUSD, expectedTotal)
	}
}

func TestCostUSDParts_UnknownModel(t *testing.T) {
	// Test unknown model returns zero costs
	inUSD, outUSD, totalUSD := CostUSDParts("unknown-model", 1000, 2000)

	if inUSD != 0 || outUSD != 0 || totalUSD != 0 {
		t.Errorf("unknown model should return zero costs: in=%f, out=%f, total=%f", inUSD, outUSD, totalUSD)
	}
}

func TestGlobalUsage_AddGetReset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Ensure clean start
	if err := ResetGlobalUsage(); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Add a couple of usage entries
	if err := AddGlobalUsage("openai", "gpt-4o", 100, 200, 0.12, 0.34); err != nil {
		t.Fatalf("add usage 1: %v", err)
	}
	if err := AddGlobalUsage("openai", "gpt-4o", 10, 20, 0.01, 0.02); err != nil {
		t.Fatalf("add usage 2: %v", err)
	}

	totals := GetGlobalUsage()

	if totals.TotalInTokens != 110 || totals.TotalOutTokens != 220 {
		t.Fatalf("token totals mismatch: in=%d out=%d", totals.TotalInTokens, totals.TotalOutTokens)
	}
	if !floatEquals(totals.TotalInUSD, 0.13, 1e-9) || !floatEquals(totals.TotalOutUSD, 0.36, 1e-9) {
		t.Fatalf("usd totals mismatch: in=%f out=%f", totals.TotalInUSD, totals.TotalOutUSD)
	}

	// Per provider
	pp, ok := totals.PerProvider["openai"]
	if !ok {
		t.Fatalf("missing provider aggregate")
	}
	if pp.InTokens != 110 || pp.OutTokens != 220 || pp.TotalTokens != 330 {
		t.Fatalf("provider tokens mismatch: %+v", pp)
	}
	if !floatEquals(pp.InUSD, 0.13, 1e-9) || !floatEquals(pp.OutUSD, 0.36, 1e-9) || !floatEquals(pp.TotalUSD, 0.49, 1e-9) {
		t.Fatalf("provider usd mismatch: %+v", pp)
	}

	// Per model
	pm, ok := totals.PerModel["gpt-4o"]
	if !ok {
		t.Fatalf("missing model aggregate")
	}
	if pm.Provider != "openai" {
		t.Fatalf("model provider mismatch: %s", pm.Provider)
	}
	if pm.InTokens != 110 || pm.OutTokens != 220 || pm.TotalTokens != 330 {
		t.Fatalf("model tokens mismatch: %+v", pm)
	}
	if !floatEquals(pm.InUSD, 0.13, 1e-9) || !floatEquals(pm.OutUSD, 0.36, 1e-9) || !floatEquals(pm.TotalUSD, 0.49, 1e-9) {
		t.Fatalf("model usd mismatch: %+v", pm)
	}

	// Reset and verify cleared
	if err := ResetGlobalUsage(); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	after := GetGlobalUsage()
	if after.TotalInTokens != 0 || after.TotalOutTokens != 0 || after.TotalInUSD != 0 || after.TotalOutUSD != 0 {
		t.Fatalf("expected zeroed totals after reset: %+v", after)
	}
}
