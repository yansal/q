package main

import (
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
	qmux "github.com/yansal/q/mux"
)

func main() {
	log.SetFlags(0)
	if err := main1(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func main1() error {
	app, err := newApp()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.Handle("/", app.mux)

	s := http.Server{
		Addr:    ":" + app.port,
		Handler: mux,
	}

	return errors.WithStack(s.ListenAndServe())
}

type app struct {
	port string
	mux  *http.ServeMux
}

func newApp() (*app, error) {
	redis, err := cmd.NewRedis()
	if err != nil {
		return nil, err
	}
	q := q.New(redis)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux, err := qmux.New(q)
	if err != nil {
		return nil, err
	}

	return &app{port: port, mux: mux}, nil
}
