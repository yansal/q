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

func receive() error {
	handlers := map[string]q.Handler{
		"debug": debugHandler,
		"error": errorHandler,
		"sleep": sleepHandler,
	}
	handlerNames := []string{"debug", "error", "sleep"}

	flagset := flag.NewFlagSet("", flag.ExitOnError)
	queue := flagset.String("queue", "", "name of the queue to receive from (required)")
	handler := flagset.String("handler", "debug", fmt.Sprintf("handler to run when a message is received -- can be one of %s", strings.Join(handlerNames, ", ")))
	flagset.Parse(os.Args[2:])

	h, ok := handlers[*handler]
	if *queue == "" || !ok {
		flagset.Usage()
		os.Exit(2)
	}

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
			return sentinelError{s}
		}
	})
	g.Go(func() error {
		return q.New(redis).Receive(ctx, *queue, h)
	})

	err = g.Wait()
	if _, ok := err.(sentinelError); ok {
		return nil
	}
	return err
}

type sentinelError struct{ os.Signal }

func (e sentinelError) Error() string { return fmt.Sprint(e.Signal) }

func debugHandler(ctx context.Context, payload string) error {
	log.Print(payload)
	return nil
}

func errorHandler(ctx context.Context, payload string) error {
	return errors.New(payload)
}

func sleepHandler(ctx context.Context, payload string) error {
	duration, err := time.ParseDuration(payload)
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
