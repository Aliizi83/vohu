package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/Aliizi83/vohu/pkg/logging"
)

// noopLogger discards everything — Worker only logs the "double failure"
// edge case (couldn't even record a job's outcome), which these tests
// never hit against fakeStore.
type noopLogger struct{}

func (noopLogger) Init()                                                                      {}
func (noopLogger) Debug(logging.Category, logging.SubCategory, string, map[string]any)        {}
func (noopLogger) Debugf(string, ...any)                                                      {}
func (noopLogger) Info(logging.Category, logging.SubCategory, string, map[string]any)         {}
func (noopLogger) Infof(string, ...any)                                                       {}
func (noopLogger) Warning(logging.Category, logging.SubCategory, string, map[string]any)      {}
func (noopLogger) Warningf(string, ...any)                                                    {}
func (noopLogger) Error(error, logging.Category, logging.SubCategory, string, map[string]any) {}
func (noopLogger) Errorf(error, string, ...any)                                               {}
func (noopLogger) Fatal(error, logging.Category, logging.SubCategory, string, map[string]any) {}
func (noopLogger) Fatalf(error, string, ...any)                                               {}

// fakeStore is an in-memory Store double — precise control over what a
// single Worker.process call sees, without a real backend's timing.
type fakeStore struct {
	mu        sync.Mutex
	enqueued  []Job
	completed []Result
}

func (s *fakeStore) Enqueue(ctx context.Context, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enqueued = append(s.enqueued, job)
	return nil
}

func (s *fakeStore) Dequeue(ctx context.Context, queues ...string) (Job, error) {
	return Job{}, errors.New("not used in these tests")
}

func (s *fakeStore) Complete(ctx context.Context, result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, result)
	return nil
}

func (s *fakeStore) AwaitResult(ctx context.Context, jobID string) (Result, error) {
	return Result{}, errors.New("not used in these tests")
}

func newTestWorker(store Store) *Worker {
	return NewWorker(store, noopLogger{}, "test-queue")
}

func TestProcess_SuccessfulHandler_RecordsCompletedResult(t *testing.T) {
	store := &fakeStore{}
	worker := newTestWorker(store)
	worker.Register("echo", func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return payload, nil
	})

	worker.process(context.Background(), Job{
		ID: "job-1", Type: "echo", Queue: "test-queue", Payload: json.RawMessage(`"hi"`),
	})

	if len(store.completed) != 1 {
		t.Fatalf("expected 1 completed result, got %d", len(store.completed))
	}
	if store.completed[0].Err != "" {
		t.Fatalf("expected no error, got %q", store.completed[0].Err)
	}
	if string(store.completed[0].Output) != `"hi"` {
		t.Fatalf("expected output %q, got %q", `"hi"`, store.completed[0].Output)
	}
	if len(store.enqueued) != 0 {
		t.Fatalf("expected no re-enqueue on success, got %d", len(store.enqueued))
	}
}

func TestProcess_FailingHandlerWithRetriesLeft_ReEnqueuesRatherThanFailing(t *testing.T) {
	store := &fakeStore{}
	worker := newTestWorker(store)
	worker.Register("flaky", func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("transient failure")
	})

	worker.process(context.Background(), Job{
		ID: "job-1", Type: "flaky", Queue: "test-queue", RetryIfFailed: 2, Attempts: 0,
	})

	if len(store.completed) != 0 {
		t.Fatalf("expected no terminal result while retries remain, got %d", len(store.completed))
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected the job to be re-enqueued once, got %d", len(store.enqueued))
	}
	if store.enqueued[0].Attempts != 1 {
		t.Fatalf("expected re-enqueued job's Attempts to be 1, got %d", store.enqueued[0].Attempts)
	}
}

func TestProcess_FailingHandlerWithNoRetriesLeft_RecordsFailedResult(t *testing.T) {
	store := &fakeStore{}
	worker := newTestWorker(store)
	worker.Register("flaky", func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("permanent failure")
	})

	// RetryIfFailed 1, already on Attempts 1 (its second and final try) —
	// after this it should NOT be re-enqueued again.
	worker.process(context.Background(), Job{
		ID: "job-1", Type: "flaky", Queue: "test-queue", RetryIfFailed: 1, Attempts: 1,
	})

	if len(store.enqueued) != 0 {
		t.Fatalf("expected no further re-enqueue once retries are exhausted, got %d", len(store.enqueued))
	}
	if len(store.completed) != 1 {
		t.Fatalf("expected exactly 1 terminal result, got %d", len(store.completed))
	}
	if store.completed[0].Err != "permanent failure" {
		t.Fatalf("expected Err %q, got %q", "permanent failure", store.completed[0].Err)
	}
}

func TestProcess_NoHandlerRegistered_RecordsFailedResultImmediately(t *testing.T) {
	store := &fakeStore{}
	worker := newTestWorker(store)

	worker.process(context.Background(), Job{ID: "job-1", Type: "unknown-type", Queue: "test-queue"})

	if len(store.completed) != 1 || store.completed[0].Err == "" {
		t.Fatalf("expected an immediate failed result for an unregistered type, got %+v", store.completed)
	}
	if len(store.enqueued) != 0 {
		t.Fatalf("expected no retry for a missing handler (RetryIfFailed defaults to 0), got %d", len(store.enqueued))
	}
}
