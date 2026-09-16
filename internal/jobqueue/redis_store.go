package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is the Store implementation used today — a plain Redis List
// per queue name (RPUSH/BLPOP) for pending jobs, plus a String key +
// Pub/Sub channel per job ID for delivering its result back to whoever's
// waiting in AwaitResult.
type RedisStore struct {
	client *redis.Client
	// blockTimeout bounds each individual BLPOP call — Dequeue loops,
	// re-checking ctx between calls, rather than issuing one call with an
	// indefinite Redis-side timeout. A cancelled ctx isn't guaranteed to
	// interrupt a BLPOP already in flight, so this is also the practical
	// upper bound on how long Dequeue takes to notice cancellation, not
	// just an anti-starvation measure.
	blockTimeout time.Duration
	// resultTTL is how long a completed job's result stays readable —
	// long enough for a slow caller to still catch it via the direct GET
	// fallback in AwaitResult, short enough that results don't accumulate
	// in Redis forever.
	resultTTL time.Duration
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client:       client,
		blockTimeout: time.Second,
		resultTTL:    10 * time.Minute,
	}
}

func pendingKey(queue string) string    { return "jobqueue:pending:" + queue }
func resultKey(jobID string) string     { return "jobqueue:result:" + jobID }
func resultChannel(jobID string) string { return "jobqueue:result:" + jobID }

func (s *RedisStore) Enqueue(ctx context.Context, job Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.client.RPush(ctx, pendingKey(job.Queue), data).Err()
}

func (s *RedisStore) Dequeue(ctx context.Context, queues ...string) (Job, error) {
	if len(queues) == 0 {
		return Job{}, errors.New("jobqueue: Dequeue needs at least one queue")
	}

	keys := make([]string, len(queues))
	for i, q := range queues {
		keys[i] = pendingKey(q)
	}

	for {
		if err := ctx.Err(); err != nil {
			return Job{}, err
		}

		result, err := s.client.BLPop(ctx, s.blockTimeout, keys...).Result()
		if errors.Is(err, redis.Nil) {
			// Nothing arrived within blockTimeout — loop back around and
			// re-check ctx rather than treating this as an error.
			continue
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Job{}, ctxErr
			}
			return Job{}, err
		}

		// result is [key, value] — BLPOP reports which of the watched keys
		// it popped from, but the job itself already carries its own
		// Queue field, so that's not needed here.
		var job Job
		if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
			return Job{}, err
		}
		return job, nil
	}
}

func (s *RedisStore) Complete(ctx context.Context, result Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, resultKey(result.JobID), data, s.resultTTL).Err(); err != nil {
		return err
	}
	// Best-effort wake-up for anyone already blocked in AwaitResult — the
	// Set above is the durable record a late/slow AwaitResult call still
	// finds via its direct GET fallback, so a Publish with no subscriber
	// isn't a problem.
	return s.client.Publish(ctx, resultChannel(result.JobID), data).Err()
}

func (s *RedisStore) AwaitResult(ctx context.Context, jobID string) (Result, error) {
	// Subscribe before checking the key — otherwise a Complete that runs
	// between the GET and the Subscribe call would be missed entirely.
	sub := s.client.Subscribe(ctx, resultChannel(jobID))
	defer sub.Close()

	if val, err := s.client.Get(ctx, resultKey(jobID)).Result(); err == nil {
		return decodeResult(val)
	} else if !errors.Is(err, redis.Nil) {
		return Result{}, err
	}

	select {
	case msg, ok := <-sub.Channel():
		if !ok {
			return Result{}, errors.New("jobqueue: result subscription closed unexpectedly")
		}
		return decodeResult(msg.Payload)
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func decodeResult(raw string) (Result, error) {
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return Result{}, err
	}
	return result, nil
}
