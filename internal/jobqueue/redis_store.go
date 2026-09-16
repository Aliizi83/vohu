package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore: a Redis List per queue (RPUSH/BLPOP) for pending jobs, plus
// a String key + Pub/Sub channel per job ID for results.
type RedisStore struct {
	client *redis.Client
	// blockTimeout bounds each BLPOP call — also the practical upper bound
	// on how long Dequeue takes to notice ctx cancellation.
	blockTimeout time.Duration
	resultTTL    time.Duration
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
			continue
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Job{}, ctxErr
			}
			return Job{}, err
		}

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
	return s.client.Publish(ctx, resultChannel(result.JobID), data).Err()
}

func (s *RedisStore) AwaitResult(ctx context.Context, jobID string) (Result, error) {
	// Subscribe before the GET check, so a Complete in between isn't missed.
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
