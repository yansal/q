package q

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

func New(client *redis.Client) Q {
	return &qredis{redis: client}
}

type qredis struct {
	redis *redis.Client
}

func (q *qredis) Publish(ctx context.Context, queue, payload string) error {
	return errors.WithStack(
		q.redis.LPush(queue, Message{
			Payload:   payload,
			CreatedAt: time.Now(),
		}).Err())
}

const (
	workers = "q:workers"
)

func (q *qredis) Receive(ctx context.Context, queue string, handler Handler) error {
	hostname, err := os.Hostname()
	if err != nil {
		return errors.WithStack(err)
	}
	self := fmt.Sprintf("%s:%d:%s:%d", hostname, os.Getpid(), queue, time.Now().UnixNano())
	processing := "q:worker:" + self

	n, err := q.redis.SAdd(workers, self).Result()
	if err != nil {
		return errors.WithStack(err)
	}
	if n != 1 {
		return errors.Errorf("expected 1 element to be added, got %d", n)
	}
	defer func() {
		n, err := q.redis.SRem(workers, self).Result()
		if err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}
		if n != 1 {
			log.Printf("%+v", errors.Errorf("expected 1 element to be removed, got %d", n))
		}
	}()

	started := "q:worker:" + self + ":started"
	if err := q.redis.Set(started, time.Now().String(), 0).Err(); err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		n, err := q.redis.Del(started).Result()
		if err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}
		if n != 1 {
			log.Printf("%+v", errors.Errorf("expected 1 key to be removed, got %d", n))
		}
	}()

	statProcessed := "q:stat:processed:" + self
	statFailed := "q:stat:failed:" + self
	defer func() {
		if err := q.redis.Del(statProcessed, statFailed).Err(); err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}
	}()

	type msg struct {
		message Message
		err     error
	}
	brpoplpush := make(chan msg)

	for {
		go func() {
			var message Message
			err := errors.WithStack(
				q.redis.BRPopLPush(queue, processing, 0).Scan(&message))
			brpoplpush <- msg{
				message: message,
				err:     err,
			}
		}()

		var message Message
		select {
		case <-ctx.Done():
			return nil
		case msg := <-brpoplpush:
			if err := msg.err; err == redis.Nil {
				continue
			} else if err != nil {
				return err
			}
			message = msg.message
		}

		// TODO: set message.RunAt?

		if err := handler(ctx, message); err != nil {
			now := time.Now()
			message := Message{
				Payload:   message.Payload,
				CreatedAt: message.CreatedAt,
				FailedAt:  &now,
			}

			if _, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
				message.Error = fmt.Sprintf("%+v", err)
			} else {
				message.Error = err.Error()
			}

			if err := errors.WithStack(
				q.redis.LPush("q:failed", message).Err()); err != nil {
				return err
			}

			if err := q.redis.Incr("q:stat:failed").Err(); err != nil {
				return errors.WithStack(err)
			}
			if err := q.redis.Incr(statFailed).Err(); err != nil {
				return errors.WithStack(err)
			}
		}

		n, err := q.redis.LRem(processing, 1, message).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		if n != 1 {
			return errors.Errorf("expected 1 element to be removed, got %d", n)
		}

		if err := q.redis.Incr("q:stat:processed").Err(); err != nil {
			return errors.WithStack(err)
		}
		if err := q.redis.Incr(statProcessed).Err(); err != nil {
			return errors.WithStack(err)
		}
	}
}

func (q *qredis) Queues() ([]Queue, int64, error) {
	smembers, err := q.redis.SMembers("resque:queues").Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	queues := make([]Queue, len(smembers))
	for i, name := range smembers {
		llen, err := q.redis.LLen("resque:queue:" + name).Result()
		if err != nil {
			return nil, 0, errors.WithStack(err)
		}
		queues[i] = Queue{Name: name, Jobs: llen}
	}

	llen, err := q.redis.LLen("resque:failed").Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return queues, llen, nil
}

func (q *qredis) Workers() ([]Worker, int64, error) {
	smembers, err := q.redis.SMembers("resque:workers").Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	if len(smembers) == 0 {
		return nil, 0, nil
	}

	const prefix = "resque:worker:"
	for i := range smembers {
		smembers[i] = prefix + smembers[i]
	}

	mget, err := q.redis.MGet(smembers...).Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	var workers []Worker
	for i := range mget {
		if mget[i] == nil {
			continue
		}
		var job jobprocessing
		if err := json.Unmarshal([]byte(mget[i].(string)), &job); err != nil {
			return nil, 0, errors.WithStack(err)
		}
		workers = append(workers, Worker{
			Where: strings.TrimPrefix(smembers[i], prefix),
			Queue: job.Queue,
			RunAt: job.RunAt,
			Class: job.Payload.Class,
		})
	}
	return workers, int64(len(smembers)), nil
}

type jobprocessing struct {
	Queue   string
	RunAt   time.Time `json:"run_at"`
	Payload struct {
		Class string
		Args  []struct{}
	}
}

func (q *qredis) Failed(offset int64) ([]Failed, int64, error) {
	const key = "resque:failed"
	llen, err := q.redis.LLen(key).Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}
	lrange, err := q.redis.LRange(key, offset, offset+19).Result()
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}
	failed := make([]Failed, len(lrange))
	for i := range lrange {
		var job jobfailed
		if err := json.Unmarshal([]byte(lrange[i]), &job); err != nil {
			return nil, 0, errors.WithStack(err)
		}
		failed[i] = Failed{
			Worker:    job.Worker,
			Queue:     job.Queue,
			FailedAt:  job.FailedAt,
			Class:     job.Payload.Class,
			Arguments: job.Payload.Args,
			Exception: job.Exception,
			Error:     job.Error,
			Backtrace: job.Backtrace,
		}
	}
	return failed, llen, nil
}

type jobfailed struct {
	FailedAt string `json:"failed_at"`
	Payload  struct {
		Class string
		Args  []struct{}
	}
	Exception string
	Error     string
	Backtrace []string
	Worker    string
	Queue     string
}
