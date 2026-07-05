package check

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func probeHTTPThroughSOCKS(ctx context.Context, socksAddress, rawURL string) (int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return 0, fmt.Errorf("unsupported probe scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return 0, fmt.Errorf("probe URL missing host")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", socksAddress)
	if err != nil {
		return 0, fmt.Errorf("connect local SOCKS: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	}

	if err := socks5Connect(conn, host, port); err != nil {
		return 0, err
	}

	var rw io.ReadWriter = conn
	if u.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return 0, fmt.Errorf("probe TLS handshake: %w", err)
		}
		defer tlsConn.Close()
		rw = tlsConn
	}

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: PX/0.1\r\nConnection: close\r\n\r\n", path, u.Host)
	if _, err := io.WriteString(rw, req); err != nil {
		return 0, fmt.Errorf("send probe request: %w", err)
	}

	reader := bufio.NewReader(rw)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read probe response: %w", err)
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("invalid HTTP status line %q", strings.TrimSpace(line))
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("invalid HTTP status code %q", fields[1])
	}
	if code < 200 || code >= 400 {
		return code, fmt.Errorf("probe returned HTTP %d", code)
	}
	return code, nil
}

func socks5Connect(conn net.Conn, host, portText string) error {
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("SOCKS greeting: %w", err)
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("SOCKS greeting response: %w", err)
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return fmt.Errorf("SOCKS no-auth rejected: %x %x", buf[0], buf[1])
	}

	port64, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid probe port %q", portText)
	}
	hostBytes := []byte(host)
	if len(hostBytes) > 255 {
		return fmt.Errorf("probe host too long")
	}

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, uint16(port64))
	req = append(req, port...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("SOCKS connect request: %w", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("SOCKS connect response: %w", err)
	}
	if header[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS version %d", header[0])
	}
	if header[1] != 0x00 {
		return fmt.Errorf("SOCKS connect failed: %s", socksReplyName(header[1]))
	}
	switch header[3] {
	case 0x01:
		_, err = io.ReadFull(conn, make([]byte, 4+2))
	case 0x03:
		length := make([]byte, 1)
		if _, err = io.ReadFull(conn, length); err == nil {
			_, err = io.ReadFull(conn, make([]byte, int(length[0])+2))
		}
	case 0x04:
		_, err = io.ReadFull(conn, make([]byte, 16+2))
	default:
		return fmt.Errorf("invalid SOCKS address type %d", header[3])
	}
	if err != nil {
		return fmt.Errorf("SOCKS bind address: %w", err)
	}
	return nil
}

func socksReplyName(code byte) string {
	names := map[byte]string{
		0x01: "general failure",
		0x02: "connection not allowed",
		0x03: "network unreachable",
		0x04: "host unreachable",
		0x05: "connection refused",
		0x06: "TTL expired",
		0x07: "command not supported",
		0x08: "address type not supported",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return fmt.Sprintf("reply code %d", code)
}
