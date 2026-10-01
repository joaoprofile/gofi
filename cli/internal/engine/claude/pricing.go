package claude

// price is a model's list price, in US dollars per million tokens.
type price struct {
	in, out, cacheRead float64
}

// prices are the list prices of the models gofi offers. They are read only
// for a model Claude Code reports with an unknown cost basis — a release
// newer than the engine, which it then prices as something else (measured:
// Sonnet 5.5 on Claude Code 2.1.283 was priced as Opus). What the engine
// knows, it prices.
var prices = map[string]price{
	"claude-fable-5-1":  {10, 50, 0.25},
	"claude-opus-5-5":   {4, 20, 0.20},
	"claude-opus-5":     {5, 25, 0.50},
	"claude-opus-4-8":   {5, 25, 0.50},
	"claude-sonnet-5-5": {2, 10, 0.20},
	"claude-sonnet-5":   {2, 10, 0.20},
	"claude-haiku-4-5":  {1, 5, 0.10},
}

// Cache writes cost a multiple of the input price, by how long they live.
const (
	cacheWrite5m = 1.25
	cacheWrite1h = 2.0
)

// modelUse is one model's share of a run, as the result line reports it.
type modelUse struct {
	Input      int     `json:"inputTokens"`
	Output     int     `json:"outputTokens"`
	CacheRead  int     `json:"cacheReadInputTokens"`
	CacheWrite int     `json:"cacheCreationInputTokens"`
	CostUSD    float64 `json:"costUSD"`
	CostBasis  string  `json:"costBasis"`
}

// cacheSplit is how the run's cache writes divide between the two lifetimes.
type cacheSplit struct {
	Ephemeral5m int `json:"ephemeral_5m_input_tokens"`
	Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
}

// runCost is what a run cost: the engine's figure for each model it knows the
// price of, and the list price for one it does not. Without a breakdown, the
// engine's total.
func runCost(total float64, byModel map[string]modelUse, split *cacheSplit) float64 {
	if len(byModel) == 0 {
		return total
	}
	// The share of cache writes kept an hour; Claude Code keeps them an hour
	// unless the split says otherwise.
	long := 1.0
	if split != nil && split.Ephemeral5m+split.Ephemeral1h > 0 {
		long = float64(split.Ephemeral1h) / float64(split.Ephemeral5m+split.Ephemeral1h)
	}
	sum := 0.0
	for model, u := range byModel {
		p, known := prices[model]
		if u.CostBasis != "unknown" || !known {
			sum += u.CostUSD
			continue
		}
		write := p.in * (long*cacheWrite1h + (1-long)*cacheWrite5m)
		sum += (float64(u.Input)*p.in + float64(u.Output)*p.out + float64(u.CacheRead)*p.cacheRead + float64(u.CacheWrite)*write) / 1e6
	}
	return sum
}
