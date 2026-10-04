package redact

import (
	"net/http"
	"testing"
)

func TestHeadersRedactsSecrets(t *testing.T) {
	headers := http.Header{}
	headers.Set("Cookie", "session=secret")
	headers.Set("Authorization", "Bearer token")
	headers.Set("X-Custom", "safe")

	got := Headers(headers)
	if got["Cookie"] != "[REDACTED]" {
		t.Fatalf("cookie was not redacted: %q", got["Cookie"])
	}
	if got["Authorization"] != "[REDACTED]" {
		t.Fatalf("authorization was not redacted: %q", got["Authorization"])
	}
	if got["X-Custom"] != "safe" {
		t.Fatalf("safe header changed: %q", got["X-Custom"])
	}
}
