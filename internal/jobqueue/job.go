// Package jobqueue is a small, backend-swappable async job queue —
// currently backed by Redis (see RedisStore), with Postgres as the
// planned next backend once durability/scale actually requires it (see
// Store's doc comment). It exists so slow work (compiling a custom tool,
// deploying it to a target host) doesn't run inline in an HTTP request's
// goroutine — see [[vohu-agent-authored-remote-automation-vision]] for
// the feature this was built for.
package jobqueue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// NewJobID returns a random, URL-safe job identifier — plain crypto/rand
// rather than a UUID library, since nothing here needs UUID's specific
// format, just a unique-enough opaque string.
func NewJobID() string {
	b := make([]byte, 16)
	// crypto/rand.Read on the standard reader never returns an error in
	// practice (it would mean the OS's CSPRNG is broken) — matched by
	// every other call site in this codebase that generates random bytes.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Job is one unit of work — plain, JSON-serializable data (never a Go
// closure) so it can round-trip through a real backend like Redis, not
// just live in process memory. Type is what a Worker's registered Handler
// dispatches on; Queue is which named queue it's stored/delivered on (a
// Worker can listen on several queues at once — see Store.Dequeue).
type Job struct {
	ID      string          `json:"id"`
	Queue   string          `json:"queue"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`

	// RetryIfFailed is how many additional attempts a failed job gets
	// before it's given up on as permanently failed — 0 means "run once,
	// never retry." Populated by the caller at enqueue time (today, from
	// config.Config.JobQueue.DefaultRetries — see cmd/server/main.go);
	// this package itself never reads config, same as every other
	// package in this codebase.
	RetryIfFailed int `json:"retryIfFailed"`
	// Attempts is how many times this job has already been dequeued and
	// run, including the current one — starts at 0, incremented by
	// Worker.Run each time it dequeues this job. A Worker re-enqueues a
	// failed job (via Store.Enqueue, same Queue) as long as
	// Attempts <= RetryIfFailed.
	Attempts int `json:"attempts"`

	CreatedAt time.Time `json:"createdAt"`
}

// Result is a completed job's outcome — Err is a plain string rather than
// the error type so it survives the same JSON round-trip Job does; empty
// means success.
type Result struct {
	JobID  string          `json:"jobId"`
	Output json.RawMessage `json:"output"`
	Err    string          `json:"err"`
}
