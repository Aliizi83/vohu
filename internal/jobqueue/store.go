package jobqueue

import "context"

// Store is the persistence + delivery abstraction behind the queue.
// RedisStore is the only implementation today; a PostgresStore can be
// added later behind the same interface.
type Store interface {
	Enqueue(ctx context.Context, job Job) error
	// Dequeue blocks until a job is available on any of the given queues,
	// or ctx is cancelled.
	Dequeue(ctx context.Context, queues ...string) (Job, error)
	Complete(ctx context.Context, result Result) error
	// AwaitResult blocks until the named job's result is recorded, or ctx
	// is cancelled.
	AwaitResult(ctx context.Context, jobID string) (Result, error)
}
