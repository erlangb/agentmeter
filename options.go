package agentmeter

// DefaultMaxHistory is the default cap on completed-run snapshots retained in History.
const DefaultMaxHistory = 100

// config holds the internal configuration for a Meter instance.
// It is populated by applying Option functions in New().
type config struct {
	costFunc   CostFunc
	maxHistory int
}

// defaultConfig returns a config with safe defaults.
func defaultConfig() config {
	return config{
		maxHistory: DefaultMaxHistory,
		costFunc:   nil, //disable cost estimation by default
	}
}

// Option is a functional option that configures a Meter instance.
// Options are applied in the order they are passed to New().
type Option func(*config)

// WithCostFunc sets the CostFunc used to estimate cost in USD for each run.
// Pass nil to disable cost estimation (the default).
func WithCostFunc(fn CostFunc) Option {
	return func(c *config) {
		c.costFunc = fn
	}
}

// WithMaxHistory sets the maximum number of completed runs to retain in history.
// n must be positive; zero or negative values are ignored and the default is kept.
func WithMaxHistory(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxHistory = n
		}
	}
}

// CostFunc computes the estimated cost in USD for a given TokenSummary.
// Implementations should be pure functions with no side effects.
// A nil CostFunc is valid; Meter will skip cost computation in that case.
type CostFunc func(TokenSummary) float64
