package config

// Price represents USD cost per token for input and output.
// Values are in USD per single token.
type Price struct {
	InPerToken  float64
	OutPerToken float64
}

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
	"gpt-4.1":      {InPerToken: 2.0 / 1e6, OutPerToken: 8.0 / 1e6},
	"o4-mini":      {InPerToken: 1.10 / 1e6, OutPerToken: 4.40 / 1e6},
	"o3":           {InPerToken: 2.0 / 1e6, OutPerToken: 8.0 / 1e6},
	"o3-mini":      {InPerToken: 1.10 / 1e6, OutPerToken: 4.40 / 1e6},
	"gpt-4.1-mini": {InPerToken: 0.40 / 1e6, OutPerToken: 1.60 / 1e6},
	"gpt-4.1-nano": {InPerToken: 0.10 / 1e6, OutPerToken: 0.40 / 1e6},
}

// CostUSDParts computes separate input and output USD costs and their sum for a given model and usage.
// Provider string is accepted for UI purposes but currently not used for pricing lookup.
func CostUSDParts(model string, inTokens, outTokens int64) (inUSD, outUSD, totalUSD float64) {
	p, ok := PricePerToken[model]
	if !ok {
		// Unknown model: default to zero pricing to avoid surprises
		return 0, 0, 0
	}
	inUSD = float64(inTokens) * p.InPerToken
	outUSD = float64(outTokens) * p.OutPerToken
	return inUSD, outUSD, inUSD + outUSD
}
