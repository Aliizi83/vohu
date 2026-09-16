package jobqueue

import "context"

// Store is the persistence + delivery abstraction behind the queue.
// RedisStore is the only implementation today; a PostgresStore (backed by
// a table and `SELECT ... FOR UPDATE SKIP LOCKED` for Dequeue) is the
// planned next one, for whenever durability across a full Redis outage or
// horizontal scaling of the store itself actually matters — nothing
// outside this package should need to change when that's added, every
// call site depends on this interface, never *RedisStore directly.
type Store interface {
	// Enqueue persists a new job for a worker to pick up later, on
	// job.Queue. Also used internally by Worker.Run to re-enqueue a
	// failed job for a retry.
	Enqueue(ctx context.Context, job Job) error

	// Dequeue blocks until a job is available on any of the given queues,
	// or ctx is cancelled — the caller (a Worker) is responsible for
	// calling Complete once it's done running the job. Listening on
	// several queues at once is a first-class case (e.g. one worker
	// handling both "custom_tool_builds" and "custom_tool_deploys"),
	// not just a single fixed queue.
	Dequeue(ctx context.Context, queues ...string) (Job, error)

	// Complete records a job's outcome and wakes up anyone blocked in
	// AwaitResult for this job ID.
	Complete(ctx context.Context, result Result) error

	// AwaitResult blocks until the named job's result is recorded, or
	// returns ctx.Err() if ctx is cancelled/times out first — this is
	// what turns "enqueue then wait" back into a synchronous-looking call
	// for a live caller (e.g. a chat tool call awaiting its custom tool's
	// result). A caller that doesn't need the result at all (e.g. a
	// future scheduled/recurring task) just never calls this.
	AwaitResult(ctx context.Context, jobID string) (Result, error)
}
