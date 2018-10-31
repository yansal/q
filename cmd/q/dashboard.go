package main

import (
	"net"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/yansal/q"
	qmux "github.com/yansal/q/mux"
)

func dashboard() error {
	redis, err := newRedis()
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
