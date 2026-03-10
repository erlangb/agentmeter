package reasoning

import "github.com/charmbracelet/lipgloss"

// PrinterStyles holds one lipgloss.Style per semantic role.
// Construct with DefaultPrinterStyles or LightPrinterStyles, then pass via WithStyles.
type PrinterStyles struct {
	Header   lipgloss.Style
	Thinking lipgloss.Style
	Call     lipgloss.Style
	Result   lipgloss.Style
	Error    lipgloss.Style
	Dim      lipgloss.Style
	Cost     lipgloss.Style
	Model    lipgloss.Style
}

// DefaultPrinterStyles returns the built-in colour scheme for dark terminal
// backgrounds: cyan bold headers, yellow thinking, green calls, magenta
// results, red errors, dark-gray dim text, yellow italic cost, bright-blue
// italic model names.
func DefaultPrinterStyles() PrinterStyles {
	return PrinterStyles{
		Header:   lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		Thinking: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		Call:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		Result:   lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		Error:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		Dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Cost:     lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true),
		Model:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Italic(true),
	}
}

// LipglossStyler wraps PrinterStyles and implements Styler using lipgloss rendering.
type LipglossStyler struct{ s PrinterStyles }

func (l LipglossStyler) Header(s string) string   { return l.s.Header.Render(s) }
func (l LipglossStyler) Dim(s string) string      { return l.s.Dim.Render(s) }
func (l LipglossStyler) Model(s string) string    { return l.s.Model.Render(s) }
func (l LipglossStyler) Cost(s string) string     { return l.s.Cost.Render(s) }
func (l LipglossStyler) Thinking(s string) string { return l.s.Thinking.Render(s) }
func (l LipglossStyler) Call(s string) string     { return l.s.Call.Render(s) }
func (l LipglossStyler) Result(s string) string   { return l.s.Result.Render(s) }
func (l LipglossStyler) Error(s string) string    { return l.s.Error.Render(s) }