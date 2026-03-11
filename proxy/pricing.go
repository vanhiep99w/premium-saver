package proxy

import "strings"

// modelPrice holds per-million-token pricing for a model.
type modelPrice struct {
	Input  float64 // price per 1M input tokens
	Output float64 // price per 1M output tokens
}

// modelPricing maps model name prefixes to their pricing.
// Prices are approximate based on public API pricing as of March 2026.
var modelPricing = map[string]modelPrice{
	// OpenAI
	"gpt-4o":      {2.50, 10.00},
	"gpt-4o-mini": {0.15, 0.60},
	"gpt-4.1":     {2.00, 8.00},
	"gpt-4.1-mini": {0.40, 1.60},
	"gpt-4.1-nano": {0.10, 0.40},
	"gpt-4-turbo": {10.00, 30.00},
	"gpt-3.5":     {0.50, 1.50},
	"o1":          {15.00, 60.00},
	"o1-mini":     {3.00, 12.00},
	"o1-pro":      {150.00, 600.00},
	"o3":          {10.00, 40.00},
	"o3-mini":     {1.10, 4.40},
	"o4-mini":     {1.10, 4.40},

	// Anthropic
	"claude-opus-4":      {15.00, 75.00},
	"claude-sonnet-4":    {3.00, 15.00},
	"claude-3.7-sonnet":  {3.00, 15.00},
	"claude-3.5-sonnet":  {3.00, 15.00},
	"claude-3.5-haiku":   {0.80, 4.00},
	"claude-3-opus":      {15.00, 75.00},
	"claude-3-sonnet":    {3.00, 15.00},
	"claude-3-haiku":     {0.25, 1.25},

	// Google
	"gemini-2.5-pro":    {1.25, 10.00},
	"gemini-2.5-flash":  {0.15, 0.60},
	"gemini-2.0-flash":  {0.10, 0.40},
	"gemini-1.5-pro":    {1.25, 5.00},
	"gemini-1.5-flash":  {0.075, 0.30},

	// xAI
	"grok-3":      {3.00, 15.00},
	"grok-3-mini": {0.30, 0.50},
	"grok-2":      {2.00, 10.00},
}

// defaultPrice is used when no matching model is found.
var defaultPrice = modelPrice{Input: 3.00, Output: 15.00}

// lookupPrice finds the best matching price for a model name.
// It tries exact match first, then prefix matching (longest prefix wins).
func lookupPrice(model string) modelPrice {
	model = strings.ToLower(model)

	// Exact match
	if p, ok := modelPricing[model]; ok {
		return p
	}

	// Prefix match — find the longest matching prefix
	var bestKey string
	for key := range modelPricing {
		if strings.HasPrefix(model, key) && len(key) > len(bestKey) {
			bestKey = key
		}
	}
	if bestKey != "" {
		return modelPricing[bestKey]
	}

	return defaultPrice
}

// CalculateCost returns the estimated cost in USD for a request.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	p := lookupPrice(model)
	cost := (float64(promptTokens) * p.Input / 1_000_000) +
		(float64(completionTokens) * p.Output / 1_000_000)
	return cost
}
