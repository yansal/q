package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
)

func main() {
	log.SetFlags(0)

	queue := flag.String("queue", "", "name of the queue to publish to (required)")
	payload := flag.String("payload", "", "payload to publish (required)")
	flag.Parse()

	if *queue == "" || *payload == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := main1(*queue, *payload); err != nil {
		log.Fatalf("%+v", err)
	}
}

func main1(queue, payload string) error {
	redis, err := cmd.NewRedis()
	if err != nil {
		return err
	}
	return q.New(redis).Publish(context.Background(), queue, payload)
}
