package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AmpedWasTaken/cbx/internal/model"
	"github.com/AmpedWasTaken/cbx/internal/redact"
	"github.com/AmpedWasTaken/cbx/internal/store"
)

const maxEventBody = 64 << 10

type eventEnvelope struct {
	Marker    string `json:"marker"`
	URL       string `json:"url"`
	Origin    string `json:"origin"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"userAgent"`
}

func New(st *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/c/", callbackHandler(st))
	return securityHeaders(mux)
}

func callbackHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/c/")
		if token == "" || strings.Contains(token, "/") {
			http.NotFound(w, r)
			return
		}

		cb, err := st.GetCallback(token)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		if cb.DisabledAt != nil || time.Now().After(cb.ExpiresAt) {
			http.Error(w, "callback expired", http.StatusGone)
			return
		}

		var envelope eventEnvelope
		var bodySize int64
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, maxEventBody+1))
			if err != nil {
				http.Error(w, "read error", http.StatusBadRequest)
				return
			}
			bodySize = int64(len(body))
			if bodySize > maxEventBody {
				http.Error(w, "event too large", http.StatusRequestEntityTooLarge)
				return
			}
			if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") && len(body) > 0 {
				_ = json.Unmarshal(body, &envelope)
			}
		}

		eventID, err := randomID(9)
		if err != nil {
			http.Error(w, "id error", http.StatusInternalServerError)
			return
		}

		userAgent := firstNonEmpty(envelope.UserAgent, r.UserAgent())
		origin := firstNonEmpty(envelope.Origin, r.Header.Get("Origin"))
		referrer := firstNonEmpty(envelope.Referrer, r.Referer())

		event := model.Event{
			ID:         eventID,
			CallbackID: cb.ID,
			ReceivedAt: time.Now().UTC(),
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			SourceIP:   remoteIP(r.RemoteAddr),
			UserAgent:  userAgent,
			Origin:     origin,
			Referrer:   referrer,
			PageURL:    envelope.URL,
			Marker:     envelope.Marker,
			Headers:    redact.Headers(r.Header),
			BodySize:   bodySize,
		}
		if cb.ExpectedHost != "" {
			match := scopeMatches(cb.ExpectedHost, envelope.URL, origin, referrer)
			event.ScopeMatch = &match
		}
		event.EvidenceHash = hashEvent(event)

		if err := st.AddEvent(event); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		if cb.Once {
			_ = st.DisableCallback(cb.ID)
		}

		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func hashEvent(event model.Event) string {
	copy := event
	copy.EvidenceHash = ""
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func randomID(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	return addr
}

func scopeMatches(expected string, values ...string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	expected = strings.TrimPrefix(expected, "*.")
	for _, value := range values {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == expected || strings.HasSuffix(host, "."+expected) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
