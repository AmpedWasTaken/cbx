package model

import "time"

type Callback struct {
	ID           string     `json:"id"`
	Name         string     `json:"name,omitempty"`
	Type         string     `json:"type"`
	ExpectedHost string     `json:"expectedHost,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	Once         bool       `json:"once"`
	DisabledAt   *time.Time `json:"disabledAt,omitempty"`
}

type Event struct {
	ID           string            `json:"id"`
	CallbackID   string            `json:"callbackId"`
	ReceivedAt   time.Time         `json:"receivedAt"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	SourceIP     string            `json:"sourceIp,omitempty"`
	UserAgent    string            `json:"userAgent,omitempty"`
	Origin       string            `json:"origin,omitempty"`
	Referrer     string            `json:"referrer,omitempty"`
	PageURL      string            `json:"pageUrl,omitempty"`
	Marker       string            `json:"marker,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodySize     int64             `json:"bodySize"`
	ScopeMatch   *bool             `json:"scopeMatch,omitempty"`
	EvidenceHash string            `json:"evidenceHash"`
}
