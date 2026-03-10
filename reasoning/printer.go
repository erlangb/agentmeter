// Package reasoning provides terminal rendering utilities for agentmeter traces.
package reasoning

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/erlangb/agentmeter"
	"github.com/samber/lo"
)

// DefaultMaxContentLen is the default maximum number of runes printed per content line.
const DefaultMaxContentLen = 400

// breakdownIndent aligns per-model breakdown lines under "[run stats] " (12 chars).
var breakdownIndent = strings.Repeat(" ", len("[run stats] "))

// printerConfig holds the internal configuration for a Printer instance.
type printerConfig struct {
	styler        Styler
	maxContentLen int
}

func defaultPrinterConfig() printerConfig {
	return printerConfig{
		styler:        LipglossStyler{DefaultPrinterStyles()},
		maxContentLen: DefaultMaxContentLen,
	}
}

// PrinterOption configures a Printer.
type PrinterOption func(*printerConfig)

// WithStyler sets the Styler used for all output.
// Use PlainStyler{} for colour-free output, or LipglossStyler{s} for custom colours.
func WithStyler(s Styler) PrinterOption {
	return func(c *printerConfig) { c.styler = s }
}

// WithStyles is a convenience wrapper around WithStyler for lipgloss-based styles.
func WithStyles(s PrinterStyles) PrinterOption {
	return WithStyler(LipglossStyler{s})
}

// WithMaxContentLen sets the maximum number of runes printed per content line.
// Content longer than this is truncated with a trailing "…".
func WithMaxContentLen(n int) PrinterOption {
	return func(c *printerConfig) { c.maxContentLen = n }
}

// Printer writes a human-readable representation of an agent's reasoning trace
// to an io.Writer. Use NewPrinter for coloured output or NewPlainPrinter for
// plain text with no ANSI codes.
type Printer struct {
	w     io.Writer
	cfg   printerConfig
	nStep int // monotonic step counter, reset by Reset() and Print()
}

// NewPrinter returns a Printer that writes to w with lipgloss colours by default.
func NewPrinter(w io.Writer, opts ...PrinterOption) *Printer {
	cfg := defaultPrinterConfig()
	for _, o := range opts {
		o(&cfg)
	}
	return &Printer{w: w, cfg: cfg}
}

// NewBufferedPrinter returns a Printer backed by a bytes.Buffer. Useful for tests.
func NewBufferedPrinter(opts ...PrinterOption) (*Printer, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	return NewPrinter(buf, opts...), buf
}

// NewPlainPrinter returns a Printer with no ANSI colour output.
func NewPlainPrinter(w io.Writer, opts ...PrinterOption) *Printer {
	return NewPrinter(w, append([]PrinterOption{WithStyler(PlainStyler{})}, opts...)...)
}

// NewBufferedPlainPrinter returns a plain Printer backed by a bytes.Buffer. Useful for tests.
func NewBufferedPlainPrinter(opts ...PrinterOption) (*Printer, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	return NewPlainPrinter(buf, opts...), buf
}

// Reset resets the step counter. Call between agent runs so that step numbering
// restarts from 1 in the next Print or Step call.
func (p *Printer) Reset() { p.nStep = 0 }

// Step renders a single reasoning step immediately and increments the step counter.
// Use this for streaming output as steps arrive rather than waiting for the full trace.
func (p *Printer) Step(s agentmeter.AgentStep) {
	p.printStep(p.nStep, s)
	p.nStep++
}

// Print renders all steps in the snapshot plus the token summary.
// The step counter resets to 0 on each call.
func (p *Printer) Print(snap agentmeter.Snapshot) {
	p.nStep = 0
	lo.ForEach(snap.Steps, func(s agentmeter.AgentStep, _ int) { p.Step(s) })
	p.PrintTokenSummary(snap)
}

// PrintTokenSummary writes the aggregate token/cost line for a snapshot.
// It is a no-op when no model calls were recorded.
func (p *Printer) PrintTokenSummary(snap agentmeter.Snapshot) {
	s := snap.TokenSummary
	if s.ModelCalls == 0 {
		return
	}
	p.printSummary(s, snap.TotalDuration)
	if len(s.ByModel) > 1 {
		p.printModelBreakdown(s.ByModel)
	}
}

// PrintHistory renders each snapshot in history then an aggregate summary across all runs.
// It is a no-op when history is empty.
func (p *Printer) PrintHistory(history []agentmeter.Snapshot) {
	if len(history) == 0 {
		return
	}
	st := p.cfg.styler
	for i, snap := range history {
		_, _ = fmt.Fprintf(p.w, "\n%s\n", st.Header(fmt.Sprintf("═══ Run %d / %d ═══", i+1, len(history))))
		p.Print(snap)
	}
	if len(history) < 2 {
		return // single run — aggregate is redundant
	}
	_, _ = fmt.Fprintf(p.w, "\n%s\n", st.Header(fmt.Sprintf("═══ Total across %d runs ═══", len(history))))
	summary, dur := aggregate(history)
	if summary.ModelCalls > 0 {
		p.printSummary(summary, dur)
		if len(summary.ByModel) > 1 {
			p.printModelBreakdown(summary.ByModel)
		}
	}
}

// PrintHistoryTokenSummary writes only the aggregate token summary across all snapshots.
// It is a no-op when history is empty or has no model calls.
func (p *Printer) PrintHistoryTokenSummary(history []agentmeter.Snapshot) {
	if len(history) == 0 {
		return
	}
	summary, dur := aggregate(history)
	if summary.ModelCalls == 0 {
		return
	}
	p.printSummary(summary, dur)
	if len(summary.ByModel) > 1 {
		p.printModelBreakdown(summary.ByModel)
	}
}

// --- internal render helpers ---

func (p *Printer) printSummary(s agentmeter.TokenSummary, dur time.Duration) {
	st := p.cfg.styler

	var modelName string
	switch len(s.ByModel) {
	case 0:
		modelName = "unknown"
	case 1:
		for k := range s.ByModel {
			modelName = k
		}
	default:
		modelName = "(multi-model)"
	}

	costStr := st.Dim("  (no cost estimate)")
	if s.EstimatedCostUSD > 0 {
		costStr = st.Cost(fmt.Sprintf("  ~$%.4f", s.EstimatedCostUSD))
	}

	_, _ = fmt.Fprintf(p.w, "%s %s · %s · %s · %s%s\n",
		st.Dim("[run stats]"),
		st.Model(modelName),
		st.Dim(fmt.Sprintf("%d call(s)", s.ModelCalls)),
		st.Dim(dur.Round(time.Millisecond).String()),
		st.Dim(formatUsage(s.AggregateTokenUsage())),
		costStr,
	)
}

func (p *Printer) printModelBreakdown(byModel map[string]agentmeter.TokenUsage) {
	st := p.cfg.styler
	for _, modelID := range slices.Sorted(maps.Keys(byModel)) {
		_, _ = fmt.Fprintf(p.w, "%s%s %s · %s\n",
			breakdownIndent,
			st.Dim("--"),
			st.Model(modelID),
			st.Dim(formatUsage(byModel[modelID])),
		)
	}
}

func (p *Printer) printHeader(i int, s agentmeter.AgentStep) {
	st := p.cfg.styler
	metrics := st.Dim(fmt.Sprintf("(%v · ↑%s ↓%s)",
		s.Duration.Round(time.Millisecond),
		formatNum(s.Usage.PromptTokens),
		formatNum(s.Usage.CompletionTokens),
	))
	_, _ = fmt.Fprintf(p.w, "\n%s %s\n",
		st.Header(fmt.Sprintf("--- step %d [%s] (%s) ---", i+1, s.Role, s.AgentName)),
		metrics,
	)
}

func (p *Printer) printStep(i int, s agentmeter.AgentStep) {
	p.printHeader(i, s)
	st := p.cfg.styler
	switch s.Cluster {
	case agentmeter.ClusterCognitive:
		p.printCognitive(s)
	case agentmeter.ClusterAction:
		label := labelResult
		if s.ToolCallID != "" {
			label = labelResult + " " + s.ToolCallID
		}
		p.line(label, st.Result, fmt.Sprintf("%s: %s %s",
			s.ToolName,
			st.Dim(fmt.Sprintf("(%db)", len(s.Content))),
			s.Content,
		))
	case agentmeter.ClusterMessage:
		p.line(labelMessage, st.Call, s.Content)
	case agentmeter.ClusterError:
		p.line(labelError, st.Error, s.Content)
	default:
		p.line(strings.ToLower(string(s.Role)), st.Dim, s.Content)
	}
}

func (p *Printer) printCognitive(s agentmeter.AgentStep) {
	st := p.cfg.styler
	if s.ThinkingContent != "" {
		p.line(labelThinking, st.Thinking, s.ThinkingContent)
	}
	hasTools := len(s.ToolCalls) > 0
	hasText := strings.TrimSpace(s.Content) != ""
	switch {
	case hasText && hasTools:
		p.line(labelPlan, st.Thinking, s.Content)
	case hasText:
		p.line(labelThought, st.Thinking, s.Content)
	}
	for _, tc := range s.ToolCalls {
		p.line(labelCall, st.Call, tc)
	}
	if hasTools && !hasText {
		_, _ = fmt.Fprintf(p.w, "  %-*s %s\n", 12, "["+labelWorking+"]", st.Dim("agent executing tools..."))
	}
}

// line prints a padded, labelled content line. styleFn is applied to the label only.
func (p *Printer) line(label string, styleFn func(string) string, text string) {
	const labelWidth = 12
	_, _ = fmt.Fprintf(p.w, "  %-*s %s\n",
		labelWidth,
		styleFn(fmt.Sprintf("[%s]", label)),
		Truncate(text, p.cfg.maxContentLen),
	)
}

// --- package-level helpers shared across the package ---

// formatUsage formats token counts as "↑N ↓N (cached:N)".
func formatUsage(u agentmeter.TokenUsage) string {
	return fmt.Sprintf("↑%s ↓%s (cached:%s)",
		formatNum(u.PromptTokens),
		formatNum(u.CompletionTokens),
		formatNum(u.CachedInputTokens),
	)
}

// formatNum formats n with thousands separators, e.g. 1_234_567 → "1,234,567".
func formatNum(n int) string {
	if n < 0 {
		return "-" + formatNum(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	return formatNum(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}

// Truncate shortens s to at most maxRunes runes, appending "…" if truncated.
func Truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// aggregate merges token usage and duration across a slice of snapshots.
func aggregate(snaps []agentmeter.Snapshot) (agentmeter.TokenSummary, time.Duration) {
	var totalDur time.Duration
	var totalCalls int
	var totalCost float64
	byModel := make(map[string]agentmeter.TokenUsage)
	for _, snap := range snaps {
		totalDur += snap.TotalDuration
		totalCalls += snap.TokenSummary.ModelCalls
		totalCost += snap.TokenSummary.EstimatedCostUSD
		for modelID, usage := range snap.TokenSummary.ByModel {
			byModel[modelID] = byModel[modelID].Add(usage)
		}
	}
	return agentmeter.TokenSummary{
		ByModel:          byModel,
		ModelCalls:       totalCalls,
		EstimatedCostUSD: totalCost,
	}, totalDur
}