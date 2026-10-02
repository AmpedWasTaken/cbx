# CBX

Disposable security callbacks from your terminal.

CBX is an open-source OAST/callback CLI for authorized web-security testing and webhook debugging. It is designed around non-destructive proof-of-execution events: create a callback, expose it through a temporary public tunnel, watch hits live, and export evidence.

> Status: early development

## Goals

- One binary
- No domain or VPS required
- Cloudflare Quick Tunnel support
- HTTP callback collection
- SQLite-backed local evidence
- XSS/SSRF/webhook proof helpers
- Automatic secret redaction
- CLI-first workflow with JSON-friendly output

## Safety model

CBX is intentionally a callback/canary collector rather than a browser C2. It does not provide remote browser command execution, cookie/session theft, arbitrary DOM exfiltration, reverse shells, or persistence.

## Planned first commands

```text
cbx serve
cbx new
cbx list
cbx inspect
cbx watch
cbx payload
cbx report
cbx doctor
```

## License

MIT
