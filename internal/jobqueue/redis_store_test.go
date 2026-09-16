package jobqueue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Aliizi83/vohu/internal/jobqueue"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) *jobqueue.RedisStore {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	return jobqueue.NewRedisStore(client)
}

func TestEnqueue_ThenDequeue_ReturnsTheSameJob(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	job := jobqueue.Job{
		ID: jobqueue.NewJobID(), Queue: "builds", Type: "custom_tool_run",
		Payload: json.RawMessage(`{"toolId":1}`),
	}
	if err := store.Enqueue(ctx, job); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	got, err := store.Dequeue(ctx, "builds")
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if got.ID != job.ID || got.Type != job.Type || string(got.Payload) != string(job.Payload) {
		t.Fatalf("expected dequeued job to match enqueued job, got %+v", got)
	}
}

func TestDequeue_ListensOnMultipleQueuesAtOnce(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	job := jobqueue.Job{ID: jobqueue.NewJobID(), Queue: "deploys", Type: "deploy"}
	if err := store.Enqueue(ctx, job); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Dequeue watches "builds" and "deploys" — the job only ever sat on
	// "deploys", so this only passes if BLPOP is actually watching both.
	got, err := store.Dequeue(ctx, "builds", "deploys")
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("expected to dequeue the job from \"deploys\", got %+v", got)
	}
}

func TestDequeue_TimesOutWhenNothingArrives(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := store.Dequeue(ctx, "empty-queue")
	if err == nil {
		t.Fatalf("expected Dequeue to return an error once ctx times out, got nil")
	}
}

func TestComplete_ThenAwaitResult_ReturnsItViaTheDirectGETFallback(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobID := jobqueue.NewJobID()
	// Complete happens *before* AwaitResult is ever called — this only
	// passes if AwaitResult's direct GET check (not just the Pub/Sub path)
	// actually works.
	if err := store.Complete(ctx, jobqueue.Result{JobID: jobID, Output: json.RawMessage(`"ok"`)}); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	result, err := store.AwaitResult(ctx, jobID)
	if err != nil {
		t.Fatalf("AwaitResult failed: %v", err)
	}
	if string(result.Output) != `"ok"` {
		t.Fatalf("expected output %q, got %q", `"ok"`, result.Output)
	}
}

func TestAwaitResult_WakesUpWhenCompleteRunsWhileWaiting(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobID := jobqueue.NewJobID()
	resultCh := make(chan jobqueue.Result, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := store.AwaitResult(ctx, jobID)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	// Give AwaitResult a moment to actually subscribe before Complete runs.
	time.Sleep(100 * time.Millisecond)

	if err := store.Complete(ctx, jobqueue.Result{JobID: jobID, Err: "boom"}); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("AwaitResult returned an error: %v", err)
	case result := <-resultCh:
		if result.Err != "boom" {
			t.Fatalf("expected Err %q, got %q", "boom", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AwaitResult never returned after Complete ran")
	}
}

func TestAwaitResult_ReturnsCtxErrOnTimeout(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := store.AwaitResult(ctx, jobqueue.NewJobID())
	if err == nil {
		t.Fatalf("expected AwaitResult to time out, got nil error")
	}
}
