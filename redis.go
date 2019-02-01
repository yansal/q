package q

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

const (
	// TODO: allow to configure the "q" namespace?
	qFailed     = "q:failed"
	qProcessing = "q:processing"
	qQueue      = "q:queues"
	qQueues     = "q:queues"
	qStats      = "q:stats"
	qWorker     = "q:worker"
	qWorkers    = "q:workers"
)

func New(client *redis.Client) Q {
	return &qredis{redis: client}
}

type qredis struct {
	redis *redis.Client
}

func (q *qredis) Receive(ctx context.Context, queue string, handler Handler) error {
	hostname, err := os.Hostname()
	if err != nil {
		return errors.WithStack(err)
	}
	name := fmt.Sprintf("%s:%d:%s:%d", hostname, os.Getpid(), queue, time.Now().UnixNano())
	self := qWorker + ":" + name
	processing := qProcessing + ":" + name

	if _, err := q.redis.SAdd(qWorkers, self).Result(); err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		if _, err := q.redis.SRem(qWorkers, self).Result(); err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}

		if _, err := q.redis.Del(self).Result(); err != nil {
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
			err := q.redis.BRPopLPush(qQueue+":"+queue, processing, 0).Scan(&message)
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
				return errors.WithStack(err)
			}
			message = msg.message
		}

		message.RunAt = newnow()
		if err := handler(ctx, message.Payload); err != nil {
			message.FailedAt = newnow()

			if _, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
				message.Error = fmt.Sprintf("%+v", err)
			} else {
				message.Error = err.Error()
			}

			if err := errors.WithStack(
				q.redis.LPush(qFailed, message).Err()); err != nil {
				return err
			}

			if err := q.redis.HIncrBy(qStats, "failed", 1).Err(); err != nil {
				return errors.WithStack(err)
			}
			if err := q.redis.HIncrBy(self, "failed", 1).Err(); err != nil {
				return errors.WithStack(err)
			}
		}

		if _, err := q.redis.Del(processing).Result(); err != nil {
			return errors.WithStack(err)
		}

		if err := q.redis.HIncrBy(self, "processed", 1).Err(); err != nil {
			return errors.WithStack(err)
		}
		if err := q.redis.HIncrBy(qStats, "processed", 1).Err(); err != nil {
			return errors.WithStack(err)
		}
	}
}

func newnow() *time.Time {
	now := time.Now()
	return &now
}

func (q *qredis) Send(ctx context.Context, queue, payload string) error {
	self := qQueue + ":" + queue
	if _, err := q.redis.SAdd(qQueues, self).Result(); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(
		q.redis.LPush(self, Message{
			Payload:   payload,
			Queue:     self,
			CreatedAt: time.Now(),
		}).Err())
}

func (q *qredis) Stats(ctx context.Context) (Stats, error) {
	var stats Stats

	members, err := q.redis.SMembers(qQueues).Result()
	if err != nil {
		return stats, errors.WithStack(err)
	}
	stats.Queues = make(map[string]int64, len(members))
	for i := range members {
		llen, err := q.redis.LLen(members[i]).Result()
		if err != nil {
			return stats, errors.WithStack(err)
		}
		stats.Queues[members[i]] = llen
	}

	processed, failed, err := q.stats(ctx, qStats)
	if err != nil {
		return stats, err
	}
	stats.Stats.Processed = processed
	stats.Stats.Failed = failed

	members, err = q.redis.SMembers(qWorkers).Result()
	if err != nil {
		return stats, errors.WithStack(err)
	}
	stats.Workers = make(map[string]Worker, len(members))
	for i := range members {
		processed, failed, err := q.stats(ctx, members[i])
		if err != nil {
			return stats, err
		}
		stats.Workers[members[i]] = Worker{
			Processed: processed,
			Failed:    failed,
		}
	}

	if err := q.redis.LRange(qFailed, 0, 20).ScanSlice(&stats.Failed); err != nil {
		return stats, errors.WithStack(err)
	}
	return stats, nil
}

func (q *qredis) stats(ctx context.Context, hash string) (int64, int64, error) {
	hmget, err := q.redis.HMGet(hash, "processed", "failed").Result()
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	processedStr, _ := hmget[0].(string)
	processed, _ := strconv.ParseInt(processedStr, 10, 64)

	failedStr, _ := hmget[1].(string)
	failed, _ := strconv.ParseInt(failedStr, 10, 64)
	return processed, failed, nil
}
