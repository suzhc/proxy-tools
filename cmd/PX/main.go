package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/suzhc/px-health/internal/backend/xray"
	"github.com/suzhc/px-health/internal/check"
	"github.com/suzhc/px-health/internal/config"
	"github.com/suzhc/px-health/internal/link"
	"github.com/suzhc/px-health/internal/output"
)

const defaultProbeURL = "https://www.gstatic.com/generate_204"
const defaultTimeout = 15 * time.Second

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func runCheck(args []string) int {
	cfg, _, err := config.LoadForExecutable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	xrayPath := cfg.XrayPath
	if env := os.Getenv("PX_XRAY_PATH"); env != "" {
		xrayPath = env
	}

	probeURL := defaultProbeURL
	if cfg.ProbeURL != "" {
		probeURL = cfg.ProbeURL
	}
	if env := os.Getenv("PX_PROBE_URL"); env != "" {
		probeURL = env
	}

	timeout := defaultTimeout
	if cfg.Timeout != "" {
		parsed, err := config.ParseTimeout(cfg.Timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		timeout = parsed
	}
	if env := os.Getenv("PX_TIMEOUT"); env != "" {
		parsed, err := config.ParseTimeout(env)
		if err != nil {
			fmt.Fprintln(os.Stderr, "PX_TIMEOUT:", err)
			return 2
		}
		timeout = parsed
	}

	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		jsonOut bool
	)

	fs.StringVar(&xrayPath, "xray", xrayPath, "path to xray executable")
	fs.StringVar(&probeURL, "probe-url", probeURL, "URL to request through the proxy")
	fs.DurationVar(&timeout, "timeout", timeout, "overall check timeout")
	fs.BoolVar(&jsonOut, "json", false, "print JSON output")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: PX check [flags] <proxy-url>")
		return 2
	}

	node, err := link.Parse(fs.Arg(0))
	if err != nil {
		result := check.NewFailedResult("parse_failed", err)
		if jsonOut {
			printJSON(result)
		} else {
			output.PrintText(os.Stdout, result)
		}
		return 2
	}

	backend := xray.New(xray.Options{Path: xrayPath})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var reporter check.Reporter
	if !jsonOut {
		reporter = output.NewTextReporter(os.Stdout)
	}

	result := check.Run(ctx, check.Options{
		Node:     node,
		ProbeURL: probeURL,
		Backend:  backend,
		Reporter: reporter,
	})

	if jsonOut {
		printJSON(result)
	}

	switch {
	case result.Status == check.StatusOK:
		return 0
	case result.Reason == "backend_missing" || result.Reason == "backend_start_failed":
		return 3
	case errors.Is(result.Err, context.DeadlineExceeded) || result.Reason == "timeout":
		return 1
	default:
		return 1
	}
}

func printJSON(result check.Result) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "PX checks network node health.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  PX check [flags] <node-url>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Example:")
	fmt.Fprintln(out, "  PX check '<node-url>'")
}
