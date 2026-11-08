// Package auth is our own login: accounts, passwords and sessions. Supabase is
// only the database. There is no email service yet, so there's no email
// check and no password reset.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warden/internal/notes"
)

// SessionLength is how long a login lasts.
const SessionLength = 30 * 24 * time.Hour

const (
	minPassword = 8
	maxPassword = 128
	maxEmail    = 254
	// rand.Text makes 26 characters. Anything much longer isn't ours.
	maxToken = 64
)

var (
	ErrBadEmail     = errors.New("not an email address")
	ErrBadPassword  = errors.New("password must be 8 to 128 characters")
	ErrBadTemplate  = errors.New("unknown template")
	ErrEmailTaken   = errors.New("this email already has an account")
	ErrWrongLogin   = errors.New("wrong email or password")
	ErrTooManyTries = errors.New("too many wrong passwords")
	ErrNoSession    = errors.New("not logged in")
)

// User is a student's account.
type User struct {
	ID       int64
	Email    string
	Template string
}

// Accounts signs students up, logs them in and finds who a session belongs to.
type Accounts struct {
	DB    *pgxpool.Pool
	tries *throttle
}

// New makes the accounts on an open database.
func New(db *pgxpool.Pool) *Accounts {
	return &Accounts{DB: db, tries: newThrottle()}
}

// Signup makes an account and logs it in, returning the session's token.
func (a *Accounts) Signup(ctx context.Context, email, password, template string) (User, string, error) {
	email, err := cleanEmail(email)
	if err != nil {
		return User{}, "", err
	}
	if n := utf8.RuneCountInString(password); n < minPassword || n > maxPassword {
		return User{}, "", ErrBadPassword
	}
	if !ValidTemplate(template) {
		return User{}, "", ErrBadTemplate
	}

	user := User{Email: email, Template: template}
	err = a.DB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, template) VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING RETURNING id`, email, hashPassword(password), template).Scan(&user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrEmailTaken
	}
	if err != nil {
		return User{}, "", err
	}
	token, err := a.newSession(ctx, user.ID)
	return user, token, err
}

// Login checks a password and returns a new session's token. A wrong email and
// a wrong password give the same error. Too many wrong passwords for this
// email or this IP address stop it before the password is even checked.
func (a *Accounts) Login(ctx context.Context, email, password, ip string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > maxEmail || utf8.RuneCountInString(password) > maxPassword {
		return User{}, "", ErrWrongLogin
	}
	if !a.tries.allowed(email, ip) {
		return User{}, "", ErrTooManyTries
	}

	var user User
	var hash string
	err := a.DB.QueryRow(ctx, `SELECT id, email, template, password_hash FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.Template, &hash)
	found := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		hash = noAccountHash()
	} else if err != nil {
		return User{}, "", err
	}
	if !checkPassword(hash, password) || !found {
		a.tries.failed(email, ip)
		return User{}, "", ErrWrongLogin
	}
	token, err := a.newSession(ctx, user.ID)
	return user, token, err
}

// Logout ends the session with this token, if there is one.
func (a *Accounts) Logout(ctx context.Context, token string) error {
	_, err := a.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// UserFor finds whose session a token is. ErrNoSession if it's unknown or expired.
// It looks the token up by its SHA-256, so how long the lookup takes says
// nothing useful about the token: nobody can choose what its hash starts with.
func (a *Accounts) UserFor(ctx context.Context, token string) (User, error) {
	if token == "" || len(token) > maxToken {
		return User{}, ErrNoSession
	}
	var user User
	err := a.DB.QueryRow(ctx, `
		SELECT u.id, u.email, u.template FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)).Scan(&user.ID, &user.Email, &user.Template)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	return user, err
}

// SetTemplate changes the template a student's next notes are made with.
func (a *Accounts) SetTemplate(ctx context.Context, userID int64, template string) (User, error) {
	if !ValidTemplate(template) {
		return User{}, ErrBadTemplate
	}
	user := User{ID: userID}
	err := a.DB.QueryRow(ctx, `UPDATE users SET template = $2 WHERE id = $1 RETURNING email, template`, userID, template).
		Scan(&user.Email, &user.Template)
	return user, err
}

// ValidTemplate says whether a student may pick this template.
func ValidTemplate(template string) bool {
	return slices.Contains(notes.Templates, template)
}

// newSession starts a session and returns its token. Only the token's hash is
// stored. The user's expired sessions are cleared out on the way.
func (a *Accounts) newSession(ctx context.Context, userID int64) (string, error) {
	token := rand.Text()
	return token, pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= now()`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, now() + make_interval(secs => $3))`,
			hashToken(token), userID, SessionLength.Seconds())
		return err
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// cleanEmail trims an email and lower-cases it, the form it's stored and
// compared in, and refuses anything that isn't a plain address.
func cleanEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if len(email) > maxEmail || at < 1 || !strings.Contains(email[at:], ".") {
		return "", ErrBadEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", ErrBadEmail
	}
	return email, nil
}
