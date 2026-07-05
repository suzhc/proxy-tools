package link

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Node struct {
	Name          string   `json:"name"`
	Protocol      string   `json:"protocol"`
	Server        string   `json:"server"`
	Port          int      `json:"port"`
	UUID          string   `json:"-"`
	Transport     string   `json:"transport,omitempty"`
	Security      string   `json:"security,omitempty"`
	Encryption    string   `json:"encryption,omitempty"`
	Flow          string   `json:"flow,omitempty"`
	SNI           string   `json:"sni,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	PublicKey     string   `json:"-"`
	ShortID       string   `json:"-"`
	SpiderX       string   `json:"spider_x,omitempty"`
	Path          string   `json:"path,omitempty"`
	Host          string   `json:"host,omitempty"`
	ALPN          []string `json:"alpn,omitempty"`
	AllowInsecure bool     `json:"allow_insecure,omitempty"`
}

func Parse(raw string) (Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, fmt.Errorf("parse URL: %w", err)
	}
	if strings.EqualFold(u.Scheme, "vless") {
		return parseVLESS(u)
	}
	return Node{}, fmt.Errorf("unsupported proxy scheme %q", u.Scheme)
}

func parseVLESS(u *url.URL) (Node, error) {
	host := u.Hostname()
	if host == "" {
		return Node{}, fmt.Errorf("missing server host")
	}
	portText := u.Port()
	if portText == "" {
		return Node{}, fmt.Errorf("missing server port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return Node{}, fmt.Errorf("invalid server port %q", portText)
	}

	q := u.Query()
	node := Node{
		Name:        fragmentName(u),
		Protocol:    "vless",
		Server:      host,
		Port:        port,
		UUID:        u.User.Username(),
		Transport:   firstNonEmpty(q.Get("type"), "tcp"),
		Security:    firstNonEmpty(q.Get("security"), "none"),
		Encryption:  firstNonEmpty(q.Get("encryption"), "none"),
		Flow:        q.Get("flow"),
		SNI:         firstNonEmpty(q.Get("sni"), q.Get("peer")),
		Fingerprint: q.Get("fp"),
		PublicKey:   q.Get("pbk"),
		ShortID:     firstNonEmpty(q.Get("sid"), q.Get("shortId")),
		SpiderX:     firstNonEmpty(q.Get("spx"), "/"),
		Path:        q.Get("path"),
		Host:        q.Get("host"),
		ALPN:        splitCSV(q.Get("alpn")),
	}

	if node.UUID == "" {
		return Node{}, fmt.Errorf("missing VLESS UUID")
	}
	if strings.EqualFold(node.Security, "reality") && node.PublicKey == "" {
		return Node{}, fmt.Errorf("missing REALITY public key (pbk)")
	}
	if raw := q.Get("allowInsecure"); raw != "" {
		node.AllowInsecure = raw == "1" || strings.EqualFold(raw, "true")
	}
	if node.Name == "" {
		node.Name = net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	}
	return node, nil
}

func fragmentName(u *url.URL) string {
	name, err := url.QueryUnescape(u.Fragment)
	if err != nil {
		return u.Fragment
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
