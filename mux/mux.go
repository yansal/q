//go:generate go run generate_embedded.go
package mux

import (
	"html/template"
	"log"
	"net/http"

	"github.com/pkg/errors"
	"github.com/yansal/q"
)

func New(q q.Q) (*http.ServeMux, error) {
	template, err := template.New("").Parse(indexHTML)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handleError(handler(q, template)))
	return mux, nil
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func handleError(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			log.Printf("%+v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func handler(q q.Q, template *template.Template) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		stats, err := q.Stats(ctx)
		if err != nil {
			return err
		}
		return errors.WithStack(template.Execute(w, stats))
	}
}
