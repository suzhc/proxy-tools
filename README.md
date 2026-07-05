# PX

PX checks whether a network node is usable.

The first version is intentionally small:

```bash
PX check '<node-url>'
```

It is not tied to any GUI client. It accepts a node URL, performs layered
diagnostics, starts a temporary backend client when needed, probes through a
local SOCKS endpoint, and then cleans up.

Text output is streamed step by step so it is clear that PX is still working.
In an interactive terminal, the temporary `...` line is replaced by the final
step result. When output is redirected, PX writes append-only lines so logs stay
readable. `--json` output is emitted once at the end so scripts can parse it
reliably.

## Build

```bash
go build -o PX ./cmd/PX
```

## Usage

```bash
./PX check '<node-url>'
```

PX can find Xray in this order:

1. `--xray /path/to/xray`
2. `PX_XRAY_PATH`
3. `xray_path` in `px.json` next to the `PX` executable
4. `xray` in `PATH`

## Config

PX automatically reads `px.json` from the same directory as the `PX` executable:

```json
{
  "xray_path": "/usr/local/bin/xray",
  "sing_box_path": "/usr/local/bin/sing-box",
  "probe_url": "https://www.gstatic.com/generate_204",
  "timeout": "15s"
}
```

With that file in place, the common command becomes:

```bash
./PX check '<node-url>'
```

Precedence is:

```text
command-line flags > environment variables > px.json > defaults/PATH
```

Supported environment variables:

- `PX_XRAY_PATH`
- `PX_PROBE_URL`
- `PX_TIMEOUT`

Optional flags:

```bash
./PX check --probe-url https://www.gstatic.com/generate_204 '<node-url>'
./PX check --timeout 20s '<node-url>'
./PX check --json '<node-url>'
```

## Exit Codes

- `0`: node is usable
- `1`: node was checked but is not usable
- `2`: invalid usage or invalid link
- `3`: backend is missing or failed to start
