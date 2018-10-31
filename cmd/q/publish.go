package main

import (
	"context"
	"flag"
	"os"

	"github.com/yansal/q"
)

func publish() error {
	flagset := flag.NewFlagSet("", flag.ExitOnError)
	queue := flagset.String("queue", "", "name of the queue to publish to (required)")
	payload := flagset.String("payload", "", "payload to publish (required)")
	flagset.Parse(os.Args[2:])

	if *queue == "" || *payload == "" {
		flagset.Usage()
		os.Exit(2)
	}

	redis, err := newRedis()
	if err != nil {
		return err
	}
	return q.New(redis).Publish(context.Background(), *queue, *payload)
}
