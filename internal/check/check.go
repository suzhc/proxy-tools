package check

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/suzhc/proxy-tools/internal/link"
)

type Options struct {
	Node     link.Node
	ProbeURL string
	Backend  Backend
	Reporter Reporter
}

func Run(ctx context.Context, opts Options) Result {
	result := NewResult(opts.Node)
	if opts.Reporter != nil {
		opts.Reporter.Start(result)
		defer func() {
			opts.Reporter.Finish(result)
		}()
	}

	if opts.ProbeURL == "" {
		result = result.fail("http_failed", fmt.Errorf("missing probe URL"))
		return result
	}
	if _, err := url.ParseRequestURI(opts.ProbeURL); err != nil {
		result = result.fail("http_failed", fmt.Errorf("invalid probe URL: %w", err))
		return result
	}

	if err := ctx.Err(); err != nil {
		result = result.fail("timeout", err)
		return result
	}

	stepStart(opts.Reporter, "dns")
	dnsStart := time.Now()
	ips, err := net.DefaultResolver.LookupHost(ctx, opts.Node.Server)
	if err != nil {
		stepDone(opts.Reporter, result.addStep("dns", StatusFailed, err.Error(), time.Since(dnsStart)))
		result = result.fail("dns_failed", err)
		return result
	}
	stepDone(opts.Reporter, result.addStep("dns", StatusOK, strings.Join(ips, ", "), time.Since(dnsStart)))

	if opts.Node.Transport == "" || strings.EqualFold(opts.Node.Transport, "tcp") || strings.EqualFold(opts.Node.Transport, "raw") {
		stepStart(opts.Reporter, "tcp")
		tcpStart := time.Now()
		dialer := net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(opts.Node.Server, strconv.Itoa(opts.Node.Port)))
		if err != nil {
			stepDone(opts.Reporter, result.addStep("tcp", StatusFailed, err.Error(), time.Since(tcpStart)))
			result = result.fail("tcp_failed", err)
			return result
		}
		_ = conn.Close()
		stepDone(opts.Reporter, result.addStep("tcp", StatusOK, "connected", time.Since(tcpStart)))
	} else {
		stepStart(opts.Reporter, "tcp")
		stepDone(opts.Reporter, result.addStep("tcp", "skipped", "transport is "+opts.Node.Transport, 0))
	}

	if opts.Backend == nil || !opts.Backend.Supports(opts.Node) {
		err := fmt.Errorf("no backend supports %s/%s", opts.Node.Protocol, opts.Node.Security)
		stepStart(opts.Reporter, "proxy")
		stepDone(opts.Reporter, result.addStep("proxy", StatusFailed, err.Error(), 0))
		result = result.fail("backend_missing", err)
		return result
	}

	stepStart(opts.Reporter, "proxy")
	backendStart := time.Now()
	proxy, err := opts.Backend.Start(ctx, opts.Node)
	if err != nil {
		stepDone(opts.Reporter, result.addStep("proxy", StatusFailed, err.Error(), time.Since(backendStart)))
		result = result.fail("backend_start_failed", err)
		return result
	}
	defer proxy.Cleanup()
	stepDone(opts.Reporter, result.addStep("proxy", StatusOK, opts.Node.Protocol+"/"+opts.Node.Security, time.Since(backendStart)))

	stepStart(opts.Reporter, "http")
	httpStart := time.Now()
	statusCode, err := probeHTTPThroughSOCKS(ctx, proxy.SOCKSAddress, opts.ProbeURL)
	if err != nil {
		stepDone(opts.Reporter, result.addStep("http", StatusFailed, err.Error(), time.Since(httpStart)))
		result = result.fail("http_failed", err)
		return result
	}
	stepDone(opts.Reporter, result.addStep("http", StatusOK, strconv.Itoa(statusCode), time.Since(httpStart)))

	result = result.ok(time.Since(result.StartedAt))
	return result
}

func stepStart(reporter Reporter, name string) {
	if reporter != nil {
		reporter.StepStart(name)
	}
}

func stepDone(reporter Reporter, step Step) {
	if reporter != nil {
		reporter.StepDone(step)
	}
}
