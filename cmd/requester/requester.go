package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/yansal/q"
)

type requester struct {
	q   q.Q
	url string
}

func (m *requester) handle(ctx context.Context, payload string) error {
	start := time.Now()
	resp, err := http.Get(m.url)
	if err != nil {
		log.Printf("duration:%s err:%v", time.Since(start), err)
	} else {
		resp.Body.Close()
		log.Printf("duration:%s status:%d", time.Since(start), resp.StatusCode)
	}

	return m.q.Send(ctx, m.url, "")
}
