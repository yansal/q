package mux

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	"github.com/yansal/q"
)

func New(q q.Q) (*http.ServeMux, error) {
	// TODO: embed template directory
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("couldn't locate templates directory")
	}
	template, err := template.New("").ParseGlob(filepath.Join(filepath.Dir(file), "templates/*.html"))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/failed", failedHandler(q, template))
	mux.Handle("/", rootHandler(q, template))
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

func failedHandler(q q.Q, template *template.Template) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, r *http.Request) error {
		var offset int64
		failed, count, err := q.Failed(offset)
		if err != nil {
			return errors.WithStack(err)
		}

		return errors.WithStack(template.ExecuteTemplate(w, "failed.html", map[string]interface{}{
			"failed": failed,
			"count":  count,
		}))
	})
}

func rootHandler(q q.Q, template *template.Template) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, r *http.Request) error {
		queues, failed, err := q.Queues()
		if err != nil {
			return errors.WithStack(err)
		}
		workers, totalWorkers, err := q.Workers()
		if err != nil {
			return errors.WithStack(err)
		}

		return errors.WithStack(template.ExecuteTemplate(w, "index.html", map[string]interface{}{
			"queues":       queues,
			"failed":       failed,
			"workers":      workers,
			"totalWorkers": totalWorkers,
		}))
	})
}
