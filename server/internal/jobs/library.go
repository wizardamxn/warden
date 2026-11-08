package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"warden/internal/limits"
	"warden/internal/notes"
	"warden/internal/pipeline"
)

// Every query here filters by the student's own user_id: one student never
// sees another's library, notes or cards.

// reviseAfter is how old a lecture must be before its cards come back in Revise.
const reviseAfter = 2 * 24 * time.Hour

// videoColumns reads a lecture's title, channel and length from its saved
// captions, as c. They're empty until the captions step is done.
// shortcut: reads them out of each saved transcript, which is large, copy them onto jobs if the library gets slow.
const videoColumns = `COALESCE(c.result->'Video'->>'Title', ''), COALESCE(c.result->'Video'->>'Channel', ''),
	COALESCE((c.result->'Video'->>'DurationSeconds')::int, 0)`

const libraryJoins = `
	FROM library l
	JOIN jobs j ON j.video_id = l.video_id AND j.template = l.template
	LEFT JOIN job_steps c ON c.job_id = j.id AND c.name = 'captions' AND c.status = 'done'`

// Lecture is one lecture in a student's library.
type Lecture struct {
	VideoID         string    `json:"video_id"`
	Title           string    `json:"title"`
	Channel         string    `json:"channel"`
	DurationSeconds int       `json:"duration_seconds"`
	Template        string    `json:"template"`
	AddedAt         time.Time `json:"added_at"`
	Status          string    `json:"status"`
}

// Card is one flashcard in Revise, with the lecture it came from.
type Card struct {
	VideoID      string    `json:"video_id"`
	Title        string    `json:"title"`
	Template     string    `json:"template"`
	AddedAt      time.Time `json:"added_at"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer"`
	StartSeconds int       `json:"start_seconds"`
}

// AddToLibrary puts a lecture in a student's library under a template, makes
// sure its job exists, and returns the job. A lecture new to the library
// counts toward the student's daily limit, and a new job toward the total cap.
// Both are checked in the same transaction that adds them.
func (q *Queue) AddToLibrary(ctx context.Context, userID int64, videoID, template string, totalCap int) (View, error) {
	err := pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		// One student's adds take turns, so two quick requests can't both pass at 2 of 3.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
			return err
		}
		var inLibrary bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM library WHERE user_id = $1 AND video_id = $2 AND template = $3)`,
			userID, videoID, template).Scan(&inLibrary)
		if err != nil {
			return err
		}
		if !inLibrary {
			if err := limits.CheckDaily(ctx, tx, userID); err != nil {
				return err
			}
			if err := limits.CheckTotal(ctx, tx, totalCap, videoID, template); err != nil {
				return err
			}
		}
		if err := enqueue(ctx, tx, videoID, template); err != nil {
			return err
		}
		// Adding it again keeps the first added_at, so Revise and the daily count stay right.
		_, err = tx.Exec(ctx, `INSERT INTO library (user_id, video_id, template) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			userID, videoID, template)
		return err
	})
	if err != nil {
		return View{}, err
	}
	return q.Get(ctx, videoID, template)
}

// LibraryTemplate says which template a lecture is in a student's library
// with, the newest if both. ErrNotFound if it isn't in their library.
func (q *Queue) LibraryTemplate(ctx context.Context, userID int64, videoID string) (string, error) {
	var template string
	err := q.DB.QueryRow(ctx, `
		SELECT template FROM library WHERE user_id = $1 AND video_id = $2
		ORDER BY added_at DESC LIMIT 1`, userID, videoID).Scan(&template)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return template, err
}

// Library lists a student's lectures, newest first. A job no worker is left
// for still shows its last status here, and ends once the lecture is opened.
func (q *Queue) Library(ctx context.Context, userID int64) ([]Lecture, error) {
	rows, err := q.DB.Query(ctx, `
		SELECT l.video_id, `+videoColumns+`, l.template, l.added_at,
			COALESCE(j.locked_until > now(), false),
			(SELECT jsonb_object_agg(s.name, s.status) FROM job_steps s WHERE s.job_id = j.id)
		`+libraryJoins+`
		WHERE l.user_id = $1
		ORDER BY l.added_at DESC, l.video_id`, userID)
	if err != nil {
		return nil, err
	}
	lectures := []Lecture{}
	for rows.Next() {
		var lecture Lecture
		var held bool
		var statuses map[string]string
		err := rows.Scan(&lecture.VideoID, &lecture.Title, &lecture.Channel, &lecture.DurationSeconds,
			&lecture.Template, &lecture.AddedAt, &held, &statuses)
		if err != nil {
			return nil, err
		}
		var steps []StepView
		for _, name := range pipeline.StepNames {
			steps = append(steps, StepView{Name: name, Status: statuses[name]})
		}
		lecture.Status = overallStatus(steps, held)
		lecture.AddedAt = lecture.AddedAt.UTC()
		lectures = append(lectures, lecture)
	}
	return lectures, rows.Err()
}

// Revise lists the flashcards from a student's lectures added more than 2 days
// ago, oldest lecture first. No algorithm, just the date.
// shortcut: sends every card at once, page it if libraries grow to thousands of cards.
func (q *Queue) Revise(ctx context.Context, userID int64) ([]Card, error) {
	rows, err := q.DB.Query(ctx, `
		SELECT l.video_id, COALESCE(c.result->'Video'->>'Title', ''), l.template, l.added_at, f.result
		`+libraryJoins+`
		JOIN job_steps f ON f.job_id = j.id AND f.name = 'flashcards' AND f.status = 'done'
		WHERE l.user_id = $1 AND l.added_at < now() - make_interval(secs => $2)
		ORDER BY l.added_at, l.video_id`, userID, reviseAfter.Seconds())
	if err != nil {
		return nil, err
	}
	cards := []Card{}
	for rows.Next() {
		var lecture Card
		var saved []byte
		if err := rows.Scan(&lecture.VideoID, &lecture.Title, &lecture.Template, &lecture.AddedAt, &saved); err != nil {
			return nil, err
		}
		var flashcards []notes.Flashcard
		if err := json.Unmarshal(saved, &flashcards); err != nil {
			return nil, err
		}
		lecture.AddedAt = lecture.AddedAt.UTC()
		for _, flashcard := range flashcards {
			card := lecture
			card.Question, card.Answer, card.StartSeconds = flashcard.Question, flashcard.Answer, flashcard.StartSeconds
			cards = append(cards, card)
		}
	}
	return cards, rows.Err()
}
