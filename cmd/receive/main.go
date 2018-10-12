package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
	"golang.org/x/sync/errgroup"
)

func main() {
	log.SetFlags(0)

	queue := flag.String("queue", "", "name of the queue to receive from (required)")
	handler := flag.String("handler", "debug", fmt.Sprintf("handler to run when a message is received -- can be one of %s", strings.Join(handlerNames, ", ")))
	flag.Parse()

	h, ok := handlers[*handler]
	if *queue == "" || !ok {
		flag.Usage()
		os.Exit(2)
	}

	if err := main1(*queue, h); err != nil {
		log.Fatalf("%+v", err)
	}
}

func main1(queue string, handler q.Handler) error {
	redis, err := cmd.NewRedis()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		select {
		case <-ctx.Done():
			return nil
		case s := <-c:
			return fmt.Errorf("%v", s)
		}
	})
	g.Go(func() error {
		return q.New(redis).Receive(ctx, queue, handler)
	})

	return g.Wait()
}

var handlers = map[string]q.Handler{
	"debug": debugHandler,
	"error": errorHandler,
	"sleep": sleepHandler,
}
var handlerNames = []string{"debug", "error", "sleep"}

func debugHandler(ctx context.Context, message q.Message) error {
	log.Printf("%+v", message)
	return nil
}

func errorHandler(ctx context.Context, message q.Message) error {
	return errors.Errorf("%+v", message)
}

func sleepHandler(ctx context.Context, message q.Message) error {
	duration, err := time.ParseDuration(message.Payload)
	if err != nil {
		return errors.WithStack(err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}
	return nil
}
