package main

import (
	"context"
	"flag"
	"os"

	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
)

func send() error {
	flagset := flag.NewFlagSet("", flag.ExitOnError)
	queue := flagset.String("queue", "", "name of the queue to send to (required)")
	payload := flagset.String("payload", "", "payload to send (required)")
	flagset.Parse(os.Args[2:])

	if *queue == "" || *payload == "" {
		flagset.Usage()
		os.Exit(2)
	}

	redis, err := cmd.NewRedis()
	if err != nil {
		return err
	}
	return q.New(redis).Send(context.Background(), *queue, *payload)
}
