package output

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"

	"github.com/suzhc/px-health/internal/check"
	"golang.org/x/term"
)

type TextReporter struct {
	w           io.Writer
	interactive bool
	pendingLine bool
}

func NewTextReporter(w io.Writer) *TextReporter {
	reporter := &TextReporter{w: w}
	if file, ok := w.(*os.File); ok {
		reporter.interactive = term.IsTerminal(int(file.Fd()))
	}
	return reporter
}

func (r *TextReporter) Start(result check.Result) {
	printHeader(r.w, result)
}

func (r *TextReporter) StepStart(name string) {
	if r.interactive {
		fmt.Fprintf(r.w, "%-10s %-7s", name, "...")
		r.pendingLine = true
		return
	}
	fmt.Fprintf(r.w, "%-10s %-7s\n", name, "...")
}

func (r *TextReporter) StepDone(step check.Step) {
	if r.interactive && r.pendingLine {
		fmt.Fprint(r.w, "\r\033[2K")
		r.pendingLine = false
	}
	printStep(r.w, step)
}

func (r *TextReporter) Finish(result check.Result) {
	if r.interactive && r.pendingLine {
		fmt.Fprintln(r.w)
		r.pendingLine = false
	}
	if len(result.Steps) > 0 {
		fmt.Fprintln(r.w)
	}
	printSummary(r.w, result)
}

func PrintText(w io.Writer, result check.Result) {
	printHeader(w, result)
	for _, step := range result.Steps {
		printStep(w, step)
	}
	if len(result.Steps) > 0 {
		fmt.Fprintln(w)
	}
	printSummary(w, result)
}

func printHeader(w io.Writer, result check.Result) {
	if result.Name != "" {
		fmt.Fprintf(w, "name: %s\n", result.Name)
	}
	if result.Protocol != "" {
		fmt.Fprintf(w, "protocol: %s\n", result.Protocol)
	}
	if result.Server != "" {
		fmt.Fprintf(w, "server: %s\n", net.JoinHostPort(result.Server, strconv.Itoa(result.Port)))
	}
	if result.Name != "" || result.Protocol != "" || result.Server != "" {
		fmt.Fprintln(w)
	}
}

func printStep(w io.Writer, step check.Step) {
	detail := step.Detail
	if step.Duration != "" {
		if detail != "" {
			detail += "  "
		}
		detail += step.Duration
	}
	fmt.Fprintf(w, "%-10s %-7s %s\n", step.Name, step.Status, detail)
}

func printSummary(w io.Writer, result check.Result) {
	fmt.Fprintf(w, "status: %s\n", result.Status)
	if result.Reason != "" {
		fmt.Fprintf(w, "reason: %s\n", result.Reason)
	}
	if result.LatencyMS > 0 {
		fmt.Fprintf(w, "latency: %dms\n", result.LatencyMS)
	}
	if result.Error != "" {
		fmt.Fprintf(w, "error: %s\n", result.Error)
	}
}
