package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/pkg/logging"
)

// Handler processes one job's payload and returns its output, or an error
// that triggers a retry if the job still has attempts left (see
// Job.RetryIfFailed).
type Handler func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error)

// Worker dequeues jobs from a fixed set of queues and dispatches them to
// a registered Handler by Job.Type, retrying up to Job.RetryIfFailed
// times (immediate re-enqueue, no backoff yet) before giving up and
// recording a failed Result.
type Worker struct {
	store    Store
	queues   []string
	handlers map[string]Handler
	logger   logging.Logger
}

func NewWorker(store Store, logger logging.Logger, queues ...string) *Worker {
	return &Worker{store: store, queues: queues, handlers: make(map[string]Handler), logger: logger}
}

// Register wires a Handler for one job Type. Registering the same Type
// twice overwrites the previous Handler — there's meant to be exactly one
// handler per type.
func (w *Worker) Register(jobType string, handler Handler) {
	w.handlers[jobType] = handler
}

// Run dequeues and processes jobs one at a time until ctx is cancelled or
// the Store itself errors (Dequeue's error, not a single job's handler
// error — a handler failure never stops the loop, see process).
func (w *Worker) Run(ctx context.Context) error {
	for {
		job, err := w.store.Dequeue(ctx, w.queues...)
		if err != nil {
			return err
		}
		w.process(ctx, job)
	}
}

func (w *Worker) process(ctx context.Context, job Job) {
	job.Attempts++

	handler, ok := w.handlers[job.Type]
	if !ok {
		w.fail(ctx, job, fmt.Errorf("jobqueue: no handler registered for job type %q", job.Type))
		return
	}

	output, err := handler(ctx, job.Payload)
	if err != nil {
		if job.Attempts <= job.RetryIfFailed {
			if enqueueErr := w.store.Enqueue(ctx, job); enqueueErr != nil {
				w.logger.Error(enqueueErr, logging.General, logging.JobQueue,
					"failed to re-enqueue job for retry", map[string]any{"jobId": job.ID, "type": job.Type})
			}
			return
		}
		w.fail(ctx, job, err)
		return
	}

	if err := w.store.Complete(ctx, Result{JobID: job.ID, Output: output}); err != nil {
		w.logger.Error(err, logging.General, logging.JobQueue,
			"failed to record job result", map[string]any{"jobId": job.ID, "type": job.Type})
	}
}

func (w *Worker) fail(ctx context.Context, job Job, err error) {
	if completeErr := w.store.Complete(ctx, Result{JobID: job.ID, Err: err.Error()}); completeErr != nil {
		w.logger.Error(completeErr, logging.General, logging.JobQueue,
			"failed to record job failure", map[string]any{"jobId": job.ID, "type": job.Type})
	}
}
