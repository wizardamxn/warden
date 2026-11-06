// Package api is the HTTP API the website calls. It only reads and writes the
// job list in Postgres. It never calls the AI and never talks to the worker.
package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"warden/internal/core"
	"warden/internal/jobs"
	"warden/internal/pipeline/captions"
)

// Plenty for one link, too small to be worth abusing.
const maxBodyBytes = 4 << 10

type server struct {
	queue *jobs.Queue
}

// New builds the API. Only webOrigin may call it from a browser, plus
// localhost when it's the local website.
func New(queue *jobs.Queue, webOrigin string) http.Handler {
	s := server{queue}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /v1/notes", s.createNote)
	mux.HandleFunc("GET /v1/notes/{video_id}", s.getNote)
	mux.HandleFunc("POST /v1/notes/{video_id}/retry", s.retryNote)

	origins := []string{webOrigin}
	if webOrigin == core.LocalWebOrigin {
		origins = append(origins, "http://localhost:3000")
	}
	return logRequests(allowOrigins(origins, mux))
}

func (s server) createNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Send a YouTube link.")
		return
	}
	id, err := captions.VideoID(body.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a YouTube video link.")
		return
	}

	view, err := s.queue.Enqueue(r.Context(), id)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	status := http.StatusAccepted
	if view.Finished() {
		status = http.StatusOK
	}
	writeJSON(w, status, view)
}

func (s server) getNote(w http.ResponseWriter, r *http.Request) {
	view, err := s.queue.Get(r.Context(), r.PathValue("video_id"))
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s server) retryNote(w http.ResponseWriter, r *http.Request) {
	view, err := s.queue.Retry(r.Context(), r.PathValue("video_id"))
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, view)
}

func (s server) writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, jobs.ErrNotFound) {
		writeError(w, http.StatusNotFound, "We don't have a note for this video.")
		return
	}
	core.Log(r.Context()).Error("request failed", "reason", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong on our side. Please try again.")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status is already sent, so a client that hung up has nothing to be told.
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// allowOrigins answers browsers. CORS alone only hides answers from other
// sites, so their requests are refused too: any page open in the student's
// browser could otherwise spend AI calls by posting to this address.
func allowOrigins(origins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !slices.Contains(origins, origin) {
				writeError(w, http.StatusForbidden, "This site can't use the API.")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter remembers the status so it can be logged.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// logRequests gives each request an ID, which also goes back to the caller, so
// a problem a student reports can be found in the logs.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := rand.Text()[:12]
		w.Header().Set("X-Request-ID", id)
		ctx := core.LogWith(r.Context(), "request_id", id)

		recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))

		log := core.Log(ctx).Info
		if recorder.status >= http.StatusInternalServerError {
			log = core.Log(ctx).Error
		}
		log("request", "method", r.Method, "path", r.URL.Path, "status", recorder.status,
			"duration", time.Since(started).Round(time.Millisecond).String())
	})
}
