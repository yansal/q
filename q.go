package q

import (
	"context"
	"encoding/json"
	"time"
)

type Q interface {
	Receive(ctx context.Context, queue string, handler Handler) error
	Send(ctx context.Context, queue, payload string) error
	Retry(ctx context.Context, id int64) error
	Stats(ctx context.Context) (Stats, error)
}

type Handler func(ctx context.Context, payload string) error

type Stats struct {
	Failed []Message
	Queues map[string]int64
	Stats  struct {
		Processed int64
		Failed    int64
	}
	Workers map[string]Worker
}

type Message struct {
	Payload   string     `json:"payload"`
	Queue     string     `json:"queue,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	RunAt     *time.Time `json:"run_at,omitempty"`
	FailedAt  *time.Time `json:"failed_at,omitempty"`
	RetriedAt *time.Time `json:"retried_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

func (message Message) MarshalBinary() ([]byte, error)     { return json.Marshal(message) }
func (message *Message) UnmarshalBinary(data []byte) error { return json.Unmarshal(data, message) }

type Worker struct {
	Processed int64
	Failed    int64
}
