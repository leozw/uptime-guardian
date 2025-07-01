package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrTimeout = errors.New("queue timeout")

type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	DomainID  string    `json:"domain_id"`
	TenantID  string    `json:"tenant_id"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
}

type RedisQueue struct {
	client    *redis.Client
	queueName string
}

func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{
		client:    client,
		queueName: "domain_checks",
	}
}

func (q *RedisQueue) Push(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Use priority as score (lower number = higher priority)
	score := float64(job.Priority)
	if score == 0 {
		score = float64(time.Now().Unix())
	}

	err = q.client.ZAdd(ctx, q.queueName, redis.Z{
		Score:  score,
		Member: data,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to push job: %w", err)
	}

	return nil
}

func (q *RedisQueue) Pop(ctx context.Context, timeout time.Duration) (*Job, error) {
	// Use BZPOPMIN for blocking pop with timeout
	result, err := q.client.BZPopMin(ctx, timeout, q.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("failed to pop job: %w", err)
	}

	if len(result) < 2 {
		return nil, errors.New("invalid result from queue")
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1].Member.(string)), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

func (q *RedisQueue) Length(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, q.queueName).Result()
}
