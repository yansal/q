package q

import (
	"context"
	"encoding/json"
	"time"
)

type Q interface {
	Publish(ctx context.Context, queue, payload string) error
	Receive(ctx context.Context, queue string, handler Handler) error

	Queues() ([]Queue, int64, error)
	Workers() ([]Worker, int64, error)
	Failed(offset int64) ([]Failed, int64, error)
}

type Handler func(ctx context.Context, message Message) error
type Message struct {
	Payload   string     `json:"payload"`
	CreatedAt time.Time  `json:"created_at"`
	FailedAt  *time.Time `json:"failed_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

func (message Message) MarshalBinary() ([]byte, error)     { return json.Marshal(message) }
func (message *Message) UnmarshalBinary(data []byte) error { return json.Unmarshal(data, message) }

type Queue struct {
	Name string
	Jobs int64
}
type Worker struct {
	Where string
	Queue string
	Class string
	RunAt time.Time
}
type Failed struct {
	Worker    string
	Queue     string
	FailedAt  string
	Class     string
	Arguments []struct{}
	Exception string
	Error     string
	Backtrace []string
}
