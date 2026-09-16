// Package jobqueue is a small, backend-swappable async job queue —
// currently backed by Redis, so slow work doesn't run inline in an HTTP
// request's goroutine.
package jobqueue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

func NewJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Job is plain JSON-serializable data, never a Go closure, so it can
// round-trip through Redis.
type Job struct {
	ID      string          `json:"id"`
	Queue   string          `json:"queue"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`

	// RetryIfFailed: additional attempts after a failure before giving up.
	RetryIfFailed int `json:"retryIfFailed"`
	Attempts      int `json:"attempts"`

	CreatedAt time.Time `json:"createdAt"`
}

// Result: Err empty means success.
type Result struct {
	JobID  string          `json:"jobId"`
	Output json.RawMessage `json:"output"`
	Err    string          `json:"err"`
}
