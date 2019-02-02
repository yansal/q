package main

import (
	"net"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
	qmux "github.com/yansal/q/mux"
)

func web() error {
	redis, err := cmd.NewRedis()
	if err != nil {
		return err
	}
	q := q.New(redis)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	qmux, err := qmux.New(q)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.Handle("/", qmux)
	s := http.Server{Handler: mux}

	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(s.Serve(l))
}
