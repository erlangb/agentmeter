package agentmeter

import (
	"sync"
	"time"

	"github.com/samber/lo"
)

// runState holds the mutable per-run fields that Reset clears.
// Separating them from Meter makes it easy to wipe a run without
// touching the accumulated history.
type runState struct {
	label         string       // human-readable run identifier, set by Reset
	startTime     time.Time    // wall-clock time of Reset(); used to compute TotalDuration
	totalDuration time.Duration // frozen at Finalize(); zero during an active run
	steps         []AgentStep
	summary       TokenSummary // accumulated incrementally by Record; zeroed by Reset
	isFinalized   bool
}

// Meter records the reasoning trace of one or more agent runs and accumulates
// a bounded history of completed runs. Safe for concurrent use.
type Meter struct {
	mu   *sync.Mutex
	cfg  config
	run  runState
	hist []Snapshot
}

// New creates a Meter instance with the supplied options applied.
// It is safe to call New with no options; all settings have safe defaults.
func New(opts ...Option) *Meter {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	return &Meter{
		mu:  &sync.Mutex{},
		cfg: cfg,
	}
}

// Reset clears the current run state and prepares Meter to record a new run.
// label is a human-readable identifier for this run (e.g. "conversation-42",
// "search-agent-run-3"). It is stored in Snapshot.Label and does not affect
// how individual steps are recorded. Steps carry their own AgentName field.
// Reset does not clear history.
func (t *Meter) Reset(label string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.run = runState{label: label, startTime: time.Now()}
}

// Record appends s to the current run and updates token aggregates.
// Duration is auto-computed from StartedAt if not set explicitly.
func (t *Meter) Record(s AgentStep) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Auto-compute step duration from StartedAt if Duration not provided.
	// StartedAt travels with the step so there is no shared timing state.
	if s.Duration == 0 && !s.StartedAt.IsZero() {
		s.Duration = time.Since(s.StartedAt)
	}

	// Process Usage — gate on ModelID so steps with prompt/completion tokens
	// but zero TotalTokens are still tracked correctly.
	if s.ModelID != "" {
		if t.run.summary.ByModel == nil {
			t.run.summary.ByModel = make(map[string]TokenUsage)
		}

		//accumulate usage by model
		t.run.summary.ByModel[s.ModelID] = t.run.summary.ByModel[s.ModelID].Add(s.Usage)
	}

	//Update ModelCalls
	if s.Cluster == ClusterCognitive {
		t.run.summary.ModelCalls++
	}

	// Commit Step
	t.run.steps = append(t.run.steps, s)
}

// Finalize marks the current run complete and appends a Snapshot to history.
// Calling it more than once on the same run is a no-op.
func (t *Meter) Finalize() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.run.isFinalized {
		return
	}
	t.run.isFinalized = true
	t.run.totalDuration = time.Since(t.run.startTime) // freeze wall-clock at finalize time

	snap := t.buildSnapshot()
	maxHistory := t.cfg.maxHistory
	if maxHistory > 0 {
		t.hist = append(t.hist, snap)
		//reslice max history
		if len(t.hist) > maxHistory {
			t.hist[0] = Snapshot{}
			t.hist = t.hist[1:]
		}
	}
}

// Snapshot returns a point-in-time, immutable view of the current run.
// The returned Snapshot is a deep copy: mutating it does not affect Meter.
// EstimatedCostUSD is computed here if a CostFunc was supplied.
func (t *Meter) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buildSnapshot()
}

// buildSnapshot builds a Snapshot from current run state. Caller must hold mu.
func (t *Meter) buildSnapshot() Snapshot {
	summary := t.run.summary
	if t.cfg.costFunc != nil {
		summary.EstimatedCostUSD = t.cfg.costFunc(summary)
	}

	// TotalDuration is wall-clock elapsed from Reset() to Finalize().
	// After Finalize it is frozen; during an active run it is computed live.
	var dur time.Duration
	if t.run.isFinalized {
		dur = t.run.totalDuration
	} else if !t.run.startTime.IsZero() {
		dur = time.Since(t.run.startTime)
	}

	return Snapshot{
		Label:         t.run.label,
		Steps:         t.run.steps,
		TokenSummary:  summary,
		TotalDuration: dur,
	}.deepCopy()
}

// History returns a copy of all completed runs in chronological order.
// Runs are added by Finalize(). The slice is bounded by the MaxHistory option
// (default 100). Use ClearHistory to reset it.
func (t *Meter) History() []Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	return lo.Map(t.hist, func(snap Snapshot, _ int) Snapshot {
		return snap.deepCopy()
	})
}

// ClearHistory removes all completed-run snapshots from memory.
func (t *Meter) ClearHistory() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hist = nil
}
