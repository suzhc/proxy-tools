# PX Design

## Goal

PX is a small command-line tool for checking whether a network node is usable.
It should not depend on any GUI client app. The first version accepts a node URL
directly and performs one active check.

```bash
PX check '<node-url>'
```

The mental model is simple: give PX one node link, and PX reports whether the
node works now, plus which layer failed when it does not.

## Language Choice

PX should be written in Go.

Reasons:

- Go builds small static binaries that are easy to ship on macOS, Linux, and
  Windows.
- The standard library is strong for CLI work, URL parsing, DNS lookup, TCP
  dialing, timeouts, JSON generation, subprocess management, and signal
  cleanup.
- PX needs to launch and supervise external proxy cores such as Xray or
  sing-box. Go is a good fit for that kind of process orchestration.
- Python would be faster to prototype, but packaging a dependable end-user CLI
  is messier.
- Rust would also work well, but it adds more implementation friction for this
  first practical tool.

PX should not reimplement protocol internals in the first version. It should
use proven backend cores to perform real protocol checks, then wrap them with a
clean diagnostic workflow.

## First Version Scope

Only one command is required:

```bash
PX check <node-url>
```

Initial protocol support:

- one supported URL-based node format
- `tcp` transport
- secure transport parameters when present

Initial backend:

- Xray

PX discovers Xray in this order:

1. `--xray /path/to/xray`
2. `PX_XRAY_PATH`
3. `xray_path` in `px.json` next to the `PX` executable
4. `xray` found in `PATH`

Bundling or auto-downloading Xray can be considered later. The first version
should be explicit and predictable.

## Command Shape

Main command:

```bash
PX check '<node-url>'
```

Useful optional flags:

```bash
PX check --probe-url https://www.gstatic.com/generate_204 '<node-url>'
PX check --timeout 10s '<node-url>'
PX check --json '<node-url>'
PX check --xray /path/to/xray '<node-url>'
```

Default probe URL:

```text
https://www.gstatic.com/generate_204
```

## Configuration File

PX should support an optional config file named `px.json` in the same directory
as the `PX` executable.

Example layout:

```text
/opt/px/
  PX
  px.json
```

Example config:

```json
{
  "xray_path": "/usr/local/bin/xray",
  "sing_box_path": "/usr/local/bin/sing-box",
  "probe_url": "https://www.gstatic.com/generate_204",
  "timeout": "15s"
}
```

This keeps common local paths out of every command:

```bash
PX check '<node-url>'
```

The config file is intentionally tied to the executable directory, not the
current working directory. This is safer and more predictable because PX may
execute programs named in the config. Running PX inside an arbitrary downloaded
folder should not cause that folder's config file to control which backend
binary gets executed.

Suggested precedence:

1. Command-line flags
2. Environment variables
3. `px.json` next to the executable
4. Built-in defaults or `PATH` discovery

For Xray:

1. `--xray`
2. `PX_XRAY_PATH`
3. `xray_path` in `px.json`
4. `xray` found in `PATH`

For sing-box, once the sing-box backend exists:

1. `--sing-box`
2. `PX_SING_BOX_PATH`
3. `sing_box_path` in `px.json`
4. `sing-box` found in `PATH`

For general check behavior:

- `--probe-url` overrides `probe_url`
- `--timeout` overrides `timeout`
- missing `probe_url` falls back to `https://www.gstatic.com/generate_204`
- missing `timeout` falls back to `15s`

Invalid config should fail clearly only when PX needs the invalid value. For
example, a bad `sing_box_path` should not break checks that use a different
backend. A malformed `timeout` should be reported as invalid configuration
because it affects all checks.

Future config commands can be added later:

```bash
PX config show
PX config init
PX check --config /path/to/px.json '<node-url>'
```

These are not required for the next implementation step.

## Diagnostic Pipeline

PX should check a node in layers:

1. Parse the share link.
2. Resolve DNS for the node server.
3. Test TCP connectivity to `server:port` when the protocol uses TCP.
4. Generate a temporary backend client config.
5. Start the backend with a local temporary SOCKS inbound on
   `127.0.0.1:<free-port>`.
6. Send an HTTP request to the probe URL through that SOCKS proxy.
7. Stop Xray and delete temporary files.
8. Print a concise result.

In the default text mode, PX should stream progress as each layer starts and
finishes. This prevents long protocol or HTTP probes from looking like a stuck
process. In an interactive terminal, the temporary `...` line should be replaced
by the final step result. When output is redirected, PX should use append-only
lines so logs do not contain terminal control sequences.

`--json` mode should remain non-streaming and print one complete JSON document
at the end. This keeps the output easy to consume from scripts.

Example success output:

```text
name: example-node
protocol: supported-protocol
server: example.net:443

dns        ...
dns        ok      203.0.113.10
tcp        ...
tcp        ok      126ms
proxy      ...
proxy      ok      supported-protocol
http       ...
http       ok      204
latency    742ms

status: ok
```

Example failure output:

```text
name: example-node
protocol: supported-protocol
server: example.net:443

dns        ok      203.0.113.10
tcp        ok      131ms
proxy      failed  backend handshake timeout

status: failed
reason: protocol_failed
```

## Result Model

Internally, each check should produce structured results even when the default
output is human-readable.

Suggested fields:

- `name`
- `protocol`
- `server`
- `port`
- `steps`
- `status`
- `reason`
- `latency_ms`
- `started_at`
- `finished_at`

Step statuses:

- `ok`
- `failed`
- `skipped`

Failure reasons:

- `parse_failed`
- `dns_failed`
- `tcp_failed`
- `backend_missing`
- `backend_start_failed`
- `protocol_failed`
- `http_failed`
- `timeout`
- `unknown`

The `--json` output should expose this structure directly.

## Link Parsing

PX should parse proxy share links into an internal `Node` model.

For the initial supported URL format:

- scheme
- user: UUID
- host: server
- port: server port
- fragment: display name
- query:
  - `type`
  - `security`
  - `encryption`
  - `flow`
  - `sni`
  - `fp`
  - `pbk`
  - `sid`
  - `spx`
  - `path`
  - `host`
  - `alpn`

PX should preserve enough fields to generate valid Xray config, but it should
redact secrets in logs and normal output.

Sensitive fields:

- UUID
- password
- public key
- short ID
- private key
- subscription URLs

## Temporary Backend Config

PX should generate a temporary backend config from the parsed node URL. The
generated config should include:

- a local SOCKS inbound on `127.0.0.1:<free-port>`
- one outbound derived from the node URL
- minimal logging
- no persisted secrets
- temporary files under the operating system temp directory

In practice Xray cannot listen on port `0` in config and report the chosen port,
so PX should first reserve a free local port, release it, and then start Xray on
that port. This has a tiny race, but it is acceptable for the first version.

## Backend Boundary

PX core should not be hard-coded to Xray forever.

Suggested package boundaries:

- `cmd/PX`: CLI entrypoint
- `internal/link`: parse share links into `Node`
- `internal/check`: diagnostic pipeline
- `internal/backend`: backend interface
- `internal/backend/xray`: Xray config generation and process runner
- `internal/output`: text and JSON formatting

Backend interface:

```go
type Backend interface {
    Supports(node Node) bool
    Start(ctx context.Context, node Node) (LocalProxy, error)
}
```

`LocalProxy` should include:

- local SOCKS address
- cleanup function
- backend logs or startup diagnostics

Later, sing-box can implement the same interface for Hysteria2, TUIC, and other
protocols.

## Exit Codes

Suggested exit codes:

- `0`: node is usable
- `1`: node checked successfully but is not usable
- `2`: invalid CLI usage or invalid link
- `3`: required backend is missing or failed to start
- `4`: unexpected internal error

This makes PX usable in scripts and future monitoring jobs.

## Future Work

After the first `check` command works:

- Add `monitor <proxy-url>` for continuous checks.
- Add `--interval`, `--fail-threshold`, and `--notify`.
- Add sing-box backend.
- Add `trojan://`, `vmess://`, `ss://`, `hysteria2://`, and `tuic://`.
- Add import helpers for subscription text, Xray JSON, sing-box JSON, Clash YAML,
  and app-specific databases.
- Add historical storage with SQLite.
- Add machine-readable event output for dashboards or alerting.
