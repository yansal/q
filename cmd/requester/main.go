//go:generate go run generate_embedded.go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"text/template"

	"github.com/pkg/errors"
	"github.com/yansal/q"
	"github.com/yansal/q/cmd"
	"golang.org/x/sync/errgroup"
)

func main() {
	redis, err := cmd.NewRedis()
	if err != nil {
		log.Fatal(err)
	}
	q := q.New(redis)

	template, err := template.New("").Parse(indexHTML)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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
		mother := mother{q: q}
		return q.Receive(ctx, motherqueue, mother.handle)
	})
	g.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/favicon.ico", http.NotFoundHandler())
		mux.Handle("/", &handler{q: q, template: template})
		s := http.Server{Addr: ":" + port, Handler: mux}

		cerr := make(chan error)
		go func() { cerr <- errors.WithStack(s.ListenAndServe()) }()
		select {
		case err := <-cerr:
			return err
		case <-ctx.Done():
			return s.Shutdown(context.Background())
		}
	})

	err = g.Wait()
	if _, ok := err.(sentinelError); !ok {
		log.Fatal(err)
	}
}

type sentinelError struct{ os.Signal }

func (e sentinelError) Error() string { return fmt.Sprint(e.Signal) }

type handler struct {
	q        q.Q
	template *template.Template
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handleError(h.serveHTTP)(w, r)
}

func handleError(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}

		herr, ok := err.(httpError)
		if !ok {
			log.Printf("%+v\n", err)
			herr = httpError{err: err, code: http.StatusInternalServerError}
		}
		if herr.err == nil {
			herr.err = errors.New(http.StatusText(herr.code))
		}
		http.Error(w, herr.Error(), herr.code)
	}
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

type httpError struct {
	err  error
	code int
}

func (e httpError) Error() string { return e.err.Error() }

func (h *handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return h.serveGET(w, r)
	case http.MethodPost:
		return h.servePOST(w, r)
	default:
		return httpError{code: http.StatusMethodNotAllowed}
	}
}

func (h *handler) serveGET(w http.ResponseWriter, r *http.Request) error {
	return errors.WithStack(h.template.Execute(w, nil))
}

func (h *handler) servePOST(w http.ResponseWriter, r *http.Request) error {
	v := r.FormValue("url")
	if v == "" {
		return httpError{err: errors.New("url is required"), code: http.StatusBadRequest}
	}
	u, err := url.Parse(v)
	if err != nil {
		return httpError{err: err, code: http.StatusBadRequest}
	}
	if u.Scheme == "" {
		return httpError{err: errors.New("invalid url"), code: http.StatusBadRequest}
	}

	payload, err := json.Marshal(motherPayload{URL: v})
	if err != nil {
		return err
	}
	if err := h.q.Send(r.Context(), motherqueue, string(payload)); err != nil {
		return err
	}

	http.Redirect(w, r, "", http.StatusFound)
	return nil
}
