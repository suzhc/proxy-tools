package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/suzhc/proxy-tools/internal/check"
	"github.com/suzhc/proxy-tools/internal/link"
)

type Options struct {
	Path string
}

type Backend struct {
	path string
}

func New(opts Options) Backend {
	return Backend{path: opts.Path}
}

func (b Backend) Supports(node link.Node) bool {
	if node.Protocol != "vless" {
		return false
	}
	transport := node.Transport
	if transport == "" {
		transport = "tcp"
	}
	return transport == "tcp" || transport == "raw"
}

func (b Backend) Start(ctx context.Context, node link.Node) (check.LocalProxy, error) {
	xrayPath, err := findXray(b.path)
	if err != nil {
		return check.LocalProxy{}, err
	}

	port, err := freePort()
	if err != nil {
		return check.LocalProxy{}, err
	}

	tempDir, err := os.MkdirTemp("", "px-xray-*")
	if err != nil {
		return check.LocalProxy{}, err
	}
	cleanupDir := func() { _ = os.RemoveAll(tempDir) }

	configPath := filepath.Join(tempDir, "config.json")
	config, err := buildConfig(node, port)
	if err != nil {
		cleanupDir()
		return check.LocalProxy{}, err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		cleanupDir()
		return check.LocalProxy{}, err
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		cleanupDir()
		return check.LocalProxy{}, err
	}

	cmd := exec.CommandContext(ctx, xrayPath, "run", "-config", configPath)
	var logs safeBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs

	if err := cmd.Start(); err != nil {
		cleanupDir()
		return check.LocalProxy{}, fmt.Errorf("start xray: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if err := waitForListen(ctx, address, done, &logs); err != nil {
		_ = cmd.Process.Kill()
		<-done
		cleanupDir()
		return check.LocalProxy{}, err
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-done:
				case <-time.After(1500 * time.Millisecond):
					_ = cmd.Process.Kill()
					<-done
				}
			}
			cleanupDir()
		})
	}

	return check.LocalProxy{SOCKSAddress: address, Cleanup: cleanup}, nil
}

func findXray(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("xray not found at %s: %w", explicit, err)
		}
		return explicit, nil
	}
	if env := os.Getenv("PX_XRAY_PATH"); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("xray not found at PX_XRAY_PATH=%s: %w", env, err)
		}
		return env, nil
	}
	if path, err := exec.LookPath("xray"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("xray executable not found; pass --xray or set PX_XRAY_PATH")
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForListen(ctx context.Context, address string, done <-chan error, logs *safeBuffer) error {
	deadline := time.Now().Add(4 * time.Second)
	for {
		select {
		case err := <-done:
			if err == nil {
				return fmt.Errorf("xray exited during startup")
			}
			return fmt.Errorf("xray exited during startup: %w; logs: %s", err, logs.String())
		default:
		}

		conn, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("xray did not listen on %s; logs: %s", address, logs.String())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

type config struct {
	Log       map[string]string `json:"log"`
	Inbounds  []inbound         `json:"inbounds"`
	Outbounds []outbound        `json:"outbounds"`
}

type inbound struct {
	Listen   string         `json:"listen"`
	Port     int            `json:"port"`
	Protocol string         `json:"protocol"`
	Settings map[string]any `json:"settings"`
}

type outbound struct {
	Tag            string         `json:"tag,omitempty"`
	Protocol       string         `json:"protocol"`
	Settings       map[string]any `json:"settings"`
	StreamSettings map[string]any `json:"streamSettings,omitempty"`
}

func buildConfig(node link.Node, localPort int) (config, error) {
	user := map[string]any{
		"id":         node.UUID,
		"encryption": firstNonEmpty(node.Encryption, "none"),
	}
	if node.Flow != "" {
		user["flow"] = node.Flow
	}

	settings := map[string]any{
		"vnext": []map[string]any{
			{
				"address": node.Server,
				"port":    node.Port,
				"users":   []map[string]any{user},
			},
		},
	}

	network := firstNonEmpty(node.Transport, "tcp")
	if network == "raw" {
		network = "tcp"
	}
	stream := map[string]any{
		"network":  network,
		"security": node.Security,
	}

	switch strings.ToLower(node.Security) {
	case "reality":
		reality := map[string]any{
			"serverName":  node.SNI,
			"fingerprint": firstNonEmpty(node.Fingerprint, "chrome"),
			"publicKey":   node.PublicKey,
			"shortId":     node.ShortID,
			"spiderX":     firstNonEmpty(node.SpiderX, "/"),
		}
		stream["realitySettings"] = reality
	case "tls":
		tlsSettings := map[string]any{
			"serverName":    node.SNI,
			"allowInsecure": node.AllowInsecure,
		}
		if node.Fingerprint != "" {
			tlsSettings["fingerprint"] = node.Fingerprint
		}
		if len(node.ALPN) > 0 {
			tlsSettings["alpn"] = node.ALPN
		}
		stream["tlsSettings"] = tlsSettings
	case "", "none":
		stream["security"] = "none"
	default:
		return config{}, fmt.Errorf("unsupported VLESS security %q", node.Security)
	}

	return config{
		Log: map[string]string{"loglevel": "warning"},
		Inbounds: []inbound{
			{
				Listen:   "127.0.0.1",
				Port:     localPort,
				Protocol: "socks",
				Settings: map[string]any{"udp": true, "auth": "noauth"},
			},
		},
		Outbounds: []outbound{
			{
				Tag:            "proxy",
				Protocol:       "vless",
				Settings:       settings,
				StreamSettings: stream,
			},
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len()+len(p) > 16*1024 {
		b.buf.Reset()
	}
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := b.buf.String()
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) > 2000 {
		return text[len(text)-2000:]
	}
	return text
}
