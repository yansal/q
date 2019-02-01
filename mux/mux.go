//go:generate go run generate_embedded.go

package mux

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	"github.com/yansal/q"
)

func New(q q.Q) (*http.ServeMux, error) {
	template, err := template.New("").Parse(indexHTML)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", &handler{q: q, template: template})
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

type handler struct {
	q        q.Q
	template *template.Template
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handleError(h.serveHTTP)(w, r)
}

func (h *handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return h.serveGET(w, r)
	case http.MethodPost:
		return h.servePOST(w, r)
	default:
		status := http.StatusMethodNotAllowed
		http.Error(w, http.StatusText(status), status)
		return nil
	}
}

func (h *handler) serveGET(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	stats, err := h.q.Stats(ctx)
	if err != nil {
		return err
	}
	return errors.WithStack(h.template.Execute(w, stats))
}

func (h *handler) servePOST(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	switch r.URL.Path {
	case "/":
		queue := r.FormValue("queue")
		payload := r.FormValue("payload")
		if err := h.q.Send(ctx, queue, payload); err != nil {
			return err
		}
	case "/retry":
		s := r.FormValue("id")
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil
		}
		if err := h.q.Retry(ctx, id); err != nil {
			return err
		}
	default:
		status := http.StatusNotFound
		http.Error(w, http.StatusText(status), status)
		return nil
	}
	http.Redirect(w, r, "", http.StatusFound)
	return nil
}
