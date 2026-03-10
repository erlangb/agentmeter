package reasoning

// Label constants used across all render paths.
const (
	labelThinking = "thinking"
	labelThought  = "thought"
	labelPlan     = "plan"
	labelCall     = "call"
	labelResult   = "result"
	labelMessage  = "message"
	labelError    = "error"
	labelWorking  = "working"
)

// Styler maps semantic roles to styled strings. Implementations may apply
// ANSI colour codes (LipglossStyler) or return the string unchanged (PlainStyler).
type Styler interface {
	Header(s string) string
	Dim(s string) string
	Model(s string) string
	Cost(s string) string
	Thinking(s string) string
	Call(s string) string
	Result(s string) string
	Error(s string) string
}

// PlainStyler implements Styler with no ANSI colour codes.
// Every method is an identity function.
type PlainStyler struct{}

func (PlainStyler) Header(s string) string   { return s }
func (PlainStyler) Dim(s string) string      { return s }
func (PlainStyler) Model(s string) string    { return s }
func (PlainStyler) Cost(s string) string     { return s }
func (PlainStyler) Thinking(s string) string { return s }
func (PlainStyler) Call(s string) string     { return s }
func (PlainStyler) Result(s string) string   { return s }
func (PlainStyler) Error(s string) string    { return s }