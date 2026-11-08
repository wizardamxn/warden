// Package api is the HTTP API the website calls. It reads and writes the job
// list and the accounts in Postgres. It never calls the AI and never talks to
// the worker.
package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"

	"warden/internal/auth"
	"warden/internal/core"
	"warden/internal/jobs"
	"warden/internal/limits"
	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

// Plenty for one link or one login, too small to be worth abusing.
const maxBodyBytes = 4 << 10

const sessionCookie = "warden_session"

type server struct {
	queue    *jobs.Queue
	accounts *auth.Accounts
	settings core.Settings
}

// New builds the API. Only settings.WebOrigin may call it from a browser, plus
// localhost when it's the local website.
func New(queue *jobs.Queue, accounts *auth.Accounts, settings core.Settings) http.Handler {
	s := server{queue, accounts, settings}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /v1/auth/signup", s.signup)
	mux.HandleFunc("POST /v1/auth/login", s.login)
	mux.HandleFunc("POST /v1/auth/logout", s.logout)
	mux.HandleFunc("GET /v1/me", s.loggedIn(s.me))
	mux.HandleFunc("PATCH /v1/me", s.loggedIn(s.setTemplate))

	mux.HandleFunc("POST /v1/notes", s.loggedIn(s.createNote))
	mux.HandleFunc("GET /v1/notes/{video_id}", s.loggedIn(s.getNote))
	mux.HandleFunc("POST /v1/notes/{video_id}/retry", s.loggedIn(s.retryNote))
	mux.HandleFunc("GET /v1/library", s.loggedIn(s.library))
	mux.HandleFunc("GET /v1/revise", s.loggedIn(s.revise))
	mux.HandleFunc("GET /v1/sample", s.sample)

	origins := []string{settings.WebOrigin}
	if settings.WebOrigin == core.LocalWebOrigin {
		origins = append(origins, "http://localhost:3000")
	}
	return logRequests(allowOrigins(origins, mux))
}

// userView is the user as the website sees it.
type userView struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Template   string `json:"template"`
	NotesToday int    `json:"notes_today"`
	DailyLimit int    `json:"daily_limit"`
}

func (s server) writeUser(w http.ResponseWriter, r *http.Request, status int, user auth.User) {
	today, err := limits.Today(r.Context(), s.queue.DB, user.ID)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, status, map[string]userView{"user": {user.ID, user.Email, user.Template, today, limits.Daily}})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Template string `json:"template"`
}

func (s server) signup(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if !readJSON(w, r, &body, "Send an email, a password and a template.") {
		return
	}
	user, token, err := s.accounts.Signup(r.Context(), body.Email, body.Password, body.Template)
	switch {
	case errors.Is(err, auth.ErrBadEmail):
		writeError(w, http.StatusBadRequest, "Enter a real email address.")
	case errors.Is(err, auth.ErrBadPassword):
		writeError(w, http.StatusBadRequest, "Use a password of 8 to 128 characters.")
	case errors.Is(err, auth.ErrBadTemplate):
		writeError(w, http.StatusBadRequest, "Pick NEET or General.")
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "This email already has an account. Log in instead.")
	case err != nil:
		s.writeFailure(w, r, err)
	default:
		setSession(w, r, token)
		s.writeUser(w, r, http.StatusCreated, user)
	}
}

func (s server) login(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if !readJSON(w, r, &body, "Send an email and a password.") {
		return
	}
	user, token, err := s.accounts.Login(r.Context(), body.Email, body.Password, clientIP(r))
	switch {
	case errors.Is(err, auth.ErrWrongLogin):
		writeError(w, http.StatusUnauthorized, "Wrong email or password.")
	case errors.Is(err, auth.ErrTooManyTries):
		writeError(w, http.StatusTooManyRequests, "Too many wrong passwords. Please wait 15 minutes and try again.")
	case err != nil:
		s.writeFailure(w, r, err)
	default:
		setSession(w, r, token)
		s.writeUser(w, r, http.StatusOK, user)
	}
}

func (s server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := s.accounts.Logout(r.Context(), cookie.Value); err != nil {
			s.writeFailure(w, r, err)
			return
		}
	}
	http.SetCookie(w, sessionCookieFor(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

// loggedIn lets a request through only with a live session, and hands the
// handler the student it belongs to.
func (s server) loggedIn(next func(http.ResponseWriter, *http.Request, auth.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user auth.User
		err := auth.ErrNoSession
		if cookie, cookieErr := r.Cookie(sessionCookie); cookieErr == nil {
			user, err = s.accounts.UserFor(r.Context(), cookie.Value)
		}
		if errors.Is(err, auth.ErrNoSession) {
			writeError(w, http.StatusUnauthorized, "Log in to continue.")
			return
		}
		if err != nil {
			s.writeFailure(w, r, err)
			return
		}
		next(w, r.WithContext(core.LogWith(r.Context(), "user_id", user.ID)), user)
	}
}

func (s server) me(w http.ResponseWriter, r *http.Request, user auth.User) {
	s.writeUser(w, r, http.StatusOK, user)
}

func (s server) setTemplate(w http.ResponseWriter, r *http.Request, user auth.User) {
	var body struct {
		Template string `json:"template"`
	}
	if !readJSON(w, r, &body, "Send a template.") {
		return
	}
	user, err := s.accounts.SetTemplate(r.Context(), user.ID, body.Template)
	if errors.Is(err, auth.ErrBadTemplate) {
		writeError(w, http.StatusBadRequest, "Pick NEET or General.")
		return
	}
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	s.writeUser(w, r, http.StatusOK, user)
}

func (s server) createNote(w http.ResponseWriter, r *http.Request, user auth.User) {
	var body struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &body, "Send a YouTube link.") {
		return
	}
	id, err := captions.VideoID(body.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a YouTube video link.")
		return
	}

	view, err := s.queue.AddToLibrary(r.Context(), user.ID, id, user.Template, s.settings.TotalDailyCap)
	switch {
	case errors.Is(err, limits.ErrDaily):
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("You've made %d notes today. More tomorrow.", limits.Daily))
		return
	case errors.Is(err, limits.ErrTotal):
		writeError(w, http.StatusServiceUnavailable, "Warden is at its limit for today. Try again tomorrow.")
		return
	case err != nil:
		s.writeFailure(w, r, err)
		return
	}
	status := http.StatusAccepted
	if view.Finished() {
		status = http.StatusOK
	}
	writeJSON(w, status, view)
}

// getNote and retryNote only find lectures in the student's own library, under
// the template they added it with.
func (s server) getNote(w http.ResponseWriter, r *http.Request, user auth.User) {
	id := r.PathValue("video_id")
	template, err := s.queue.LibraryTemplate(r.Context(), user.ID, id)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	view, err := s.queue.Get(r.Context(), id, template)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// retryNote never counts toward the daily limit: the lecture is already in the library.
func (s server) retryNote(w http.ResponseWriter, r *http.Request, user auth.User) {
	id := r.PathValue("video_id")
	template, err := s.queue.LibraryTemplate(r.Context(), user.ID, id)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	view, err := s.queue.Retry(r.Context(), id, template)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, view)
}

func (s server) library(w http.ResponseWriter, r *http.Request, user auth.User) {
	lectures, err := s.queue.Library(r.Context(), user.ID)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]jobs.Lecture{"notes": lectures})
}

func (s server) revise(w http.ResponseWriter, r *http.Request, user auth.User) {
	cards, err := s.queue.Revise(r.Context(), user.ID)
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]jobs.Card{"cards": cards})
}

// sample is the one note anyone can open, with no login.
func (s server) sample(w http.ResponseWriter, r *http.Request) {
	if s.settings.SampleVideoID == "" {
		writeError(w, http.StatusNotFound, "There's no sample lecture yet.")
		return
	}
	view, err := s.queue.Get(r.Context(), s.settings.SampleVideoID, notes.NEET)
	if errors.Is(err, jobs.ErrNotFound) {
		writeError(w, http.StatusNotFound, "There's no sample lecture yet.")
		return
	}
	if err != nil {
		s.writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s server) writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, jobs.ErrNotFound) {
		writeError(w, http.StatusNotFound, "We don't have a note for this video.")
		return
	}
	core.Log(r.Context()).Error("request failed", "reason", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong on our side. Please try again.")
}

// readJSON reads a small JSON body, or answers 400 with problem.
func readJSON(w http.ResponseWriter, r *http.Request, into any, problem string) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, problem)
		return false
	}
	return true
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

func setSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, sessionCookieFor(r, token, int(auth.SessionLength.Seconds())))
}

// sessionCookieFor is the session cookie: out of reach of the page's scripts,
// not sent along by other sites' forms, and HTTPS only except on this computer,
// where the API runs on plain http. maxAge -1 deletes it.
func sessionCookieFor(r *http.Request, token string, maxAge int) *http.Cookie {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	local := host == "127.0.0.1" || host == "localhost"
	return &http.Cookie{Name: sessionCookie, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: !local}
}

// clientIP is the address a request came from, for login throttling. Behind
// Cloudflare Tunnel every request reaches us from this machine, and Cloudflare
// puts the real address in CF-Connecting-IP.
// shortcut: trusts CF-Connecting-IP on requests from this machine only, check this again if anything but the tunnel ever sits in front of the API.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if forwarded := r.Header.Get("CF-Connecting-IP"); net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	return host
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
			// The website sends the session cookie with every call.
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
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
// a problem a student reports can be found in the logs. Bodies, cookies and
// headers are never logged: they hold passwords and session tokens.
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
