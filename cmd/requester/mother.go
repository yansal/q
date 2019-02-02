package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/yansal/q"
)

const motherqueue = "mother"

type motherPayload struct {
	URL string `json:"url"`
}

type mother struct{ q q.Q }

func (m *mother) handle(ctx context.Context, payload string) error {
	var p motherPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		log.Fatal(err)
	}

	go func() {
		requester := requester{url: p.URL, q: m.q}
		if err := m.q.Receive(ctx, p.URL, requester.handle); err != nil {
			// TODO: restart?
			log.Print(err)
		}
	}()

	return m.q.Send(ctx, p.URL, "")
}
