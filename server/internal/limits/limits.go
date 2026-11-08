// Package limits keeps the AI bill capped: a daily limit for each student, and
// a total across everyone.
package limits

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Daily is how many new lectures a student may add each day.
const Daily = 3

// Any fixed number, so two new jobs check the total cap in turn.
const totalLockID = 8_100_418

var (
	ErrDaily = errors.New("the student's daily limit is reached")
	ErrTotal = errors.New("the total daily cap is reached")
)

// startOfToday is midnight in India, where the students are, so "today" ends
// at their midnight rather than the server's.
const startOfToday = `(date_trunc('day', now() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata')`

// querier is a database or a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Today counts the lectures a student added to their library today. A lecture
// added again keeps its first time, and Try again adds nothing, so neither counts.
func Today(ctx context.Context, db querier, userID int64) (int, error) {
	var count int
	err := db.QueryRow(ctx, `SELECT count(*) FROM library WHERE user_id = $1 AND added_at >= `+startOfToday, userID).Scan(&count)
	return count, err
}

// CheckDaily refuses one more new lecture once a student has added Daily
// today. The caller holds the student's row locked until it has added the
// lecture, so two quick requests at 2 of 3 can't both pass.
func CheckDaily(ctx context.Context, tx pgx.Tx, userID int64) error {
	count, err := Today(ctx, tx, userID)
	if err != nil {
		return err
	}
	if count >= Daily {
		return ErrDaily
	}
	return nil
}

// CheckTotal refuses a new job once totalCap jobs were made today, counting
// every student. A job that already exists costs nothing more, so it passes.
// 0 means no cap. The lock is held until the caller's transaction ends, so two
// new jobs at the cap's last place can't both pass.
func CheckTotal(ctx context.Context, tx pgx.Tx, totalCap int, videoID, template string) error {
	if totalCap <= 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, totalLockID); err != nil {
		return err
	}
	var exists bool
	var count int
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM jobs WHERE video_id = $1 AND template = $2),
			(SELECT count(*) FROM jobs WHERE created_at >= `+startOfToday+`)`, videoID, template).Scan(&exists, &count)
	if err != nil {
		return err
	}
	if !exists && count >= totalCap {
		return ErrTotal
	}
	return nil
}
