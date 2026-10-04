package redact

import (
	"net/http"
	"strings"
)

var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"x-auth-token":        {},
}

func Headers(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for name, values := range headers {
		if _, sensitive := sensitiveHeaders[strings.ToLower(name)]; sensitive {
			out[name] = "[REDACTED]"
			continue
		}
		value := strings.Join(values, ", ")
		if looksSecret(value) {
			out[name] = "[REDACTED]"
			continue
		}
		out[name] = value
	}
	return out
}

func looksSecret(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lower, "bearer ") {
		return true
	}
	for _, prefix := range []string{"sk-", "ghp_", "github_pat_", "xoxb-", "xoxp-"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(value, "AKIA") && len(value) >= 16
}
