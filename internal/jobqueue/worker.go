package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/pkg/logging"
)

type Handler func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error)

// Worker dequeues jobs and dispatches them by Job.Type, retrying up to
// Job.RetryIfFailed times before recording a failed Result.
type Worker struct {
	store    Store
	queues   []string
	handlers map[string]Handler
	logger   logging.Logger
}

func NewWorker(store Store, logger logging.Logger, queues ...string) *Worker {
	return &Worker{store: store, queues: queues, handlers: make(map[string]Handler), logger: logger}
}

func (w *Worker) Register(jobType string, handler Handler) {
	w.handlers[jobType] = handler
}

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
