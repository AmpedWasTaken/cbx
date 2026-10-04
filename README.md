# CBX

**Disposable OAST callbacks from your terminal.**

CBX is a small Go CLI for authorized web-security testing and webhook debugging. It starts a local callback collector, can expose it through a free Cloudflare Quick Tunnel, creates disposable callback URLs, records proof events locally, and turns them into report-friendly evidence.

> Status: early MVP. The current branch is functional and CI-tested.

## Why CBX?

- No domain required
- No VPS required
- No CBX account required
- Single Go binary
- Temporary public URL through Cloudflare Quick Tunnels
- XSS, SSRF, HTTP and webhook callback helpers
- Automatic redaction of common secret-bearing headers
- SHA-256 evidence hashes
- Callback TTLs and one-shot callbacks
- Expected-host correlation
- JSON-friendly output for scripting

CBX is intentionally a callback/canary collector rather than a browser C2. It does not provide remote browser command execution, cookie/session theft, arbitrary DOM exfiltration, reverse shells, or persistence.

## Build

Requirements:

- Go 1.22+
- cloudflared in PATH for public mode

```bash
git clone https://github.com/AmpedWasTaken/cbx.git
cd cbx
git checkout feat/mvp-callback-cli
go build -o cbx ./cmd/cbx
```

## First run

Check your environment:

```bash
./cbx doctor
```

Start a public collector:

```bash
./cbx serve
```

Cloudflare Quick Tunnel provides the temporary public hostname. To stay local:

```bash
./cbx serve --local
```

In another terminal create a callback:

```bash
./cbx new --name reflected-search --type xss --host staging.example.com
```

Example output:

```text
Callback created

ID       Qf0jzv7p6HW5Qy9k
Name     reflected-search
Type     xss
TTL      1h0m0s
Scope    staging.example.com
URL      https://random-words.trycloudflare.com/c/Qf0jzv7p6HW5Qy9k
```

Generate a non-destructive proof callback:

```bash
./cbx payload Qf0jzv7p6HW5Qy9k --type xss-event
```

Watch incoming events:

```bash
./cbx watch
```

Inspect or produce Markdown evidence:

```bash
./cbx inspect Qf0jzv7p6HW5Qy9k
./cbx report Qf0jzv7p6HW5Qy9k
```

## Commands

```text
cbx serve
cbx new
cbx list
cbx inspect <id>
cbx watch
cbx payload <id>
cbx report <id>
cbx doctor
cbx version
```

### Disposable callbacks

```bash
cbx new \
  --name search-xss \
  --type xss \
  --host staging.example.com \
  --ttl 30m \
  --once
```

The expected host is correlation metadata. It helps flag callbacks that appear to originate from an unexpected host without turning CBX into an intrusive agent.

### Payload helpers

```bash
cbx payload <id> --type xss-fetch
cbx payload <id> --type xss-event
cbx payload <id> --type xss-img
cbx payload <id> --type ssrf
cbx payload <id> --type webhook
cbx payload <id> --type curl
```

The xss-event helper submits only a CBX marker plus page URL, origin, referrer and user agent. It does not collect cookies, local storage, authentication tokens or DOM content.

## Evidence handling

Incoming events are stored under:

```text
~/.cbx/data/
```

Override the data location with CBX_HOME:

```bash
CBX_HOME=/tmp/cbx-lab ./cbx serve --local
```

The MVP deliberately uses small atomic JSON files so it remains dependency-free. A SQLite backend is planned before the first stable release.

Common secret-bearing headers such as Authorization, Cookie, Set-Cookie and X-Api-Key are redacted before events are persisted. Each stored event also receives a SHA-256 evidence hash.

## Current roadmap

- [x] HTTP callback collector
- [x] Cloudflare Quick Tunnel integration
- [x] Disposable cryptographic callback IDs
- [x] TTL and one-shot callbacks
- [x] Secret header redaction
- [x] Evidence hashing
- [x] XSS/SSRF/webhook proof helpers
- [x] Live event watcher
- [x] Markdown evidence reports
- [x] CI build, test and vet
- [ ] SQLite backend
- [ ] Bubble Tea TUI
- [ ] QR output
- [ ] HTML/JSON report export
- [ ] wait/assert CI commands
- [ ] Optional localhost web UI
- [ ] Cross-platform release binaries

## Safety

Use CBX only on systems you own or are explicitly authorized to test.

See SECURITY.md for reporting issues in CBX itself.

## License

MIT
