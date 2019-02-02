package main

import (
	"context"
	"fmt"

	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
)

func stats() error {
	redis, err := cmd.NewRedis()
	if err != nil {
		return err
	}
	stats, err := q.New(redis).Stats(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", stats)
	return nil
}
