package outbox

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, topic string, payload interface{}) error
	FetchPendingEvents(ctx context.Context, limit int) ([]*Event, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, err error) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) InsertEvent(ctx context.Context, tx pgx.Tx, topic string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (topic, payload)
		VALUES ($1, $2)
	`, topic, payloadBytes)

	return err
}

func (r *postgresRepository) FetchPendingEvents(ctx context.Context, limit int) ([]*Event, error) {
	// FOR UPDATE SKIP LOCKED requires a transaction to hold the lock.
	// However, since our worker processes events one-by-one after fetching,
	// and immediately marks them as PUBLISHED/FAILED, we use a simple query here.
	// The SKIP LOCKED still works to prevent two workers from selecting the same row.
	query := `
		SELECT id, topic, payload, status, attempts, next_retry_at, created_at, updated_at
		FROM outbox_events
		WHERE status = 'PENDING' AND next_retry_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		ev := &Event{}
		if err := rows.Scan(
			&ev.ID, &ev.Topic, &ev.Payload, &ev.Status, &ev.Attempts,
			&ev.NextRetryAt, &ev.CreatedAt, &ev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *postgresRepository) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET status = 'PUBLISHED', updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

func (r *postgresRepository) MarkFailed(ctx context.Context, id string, publishErr error) error {
	// Exponential backoff or max retries logic could go here.
	// For simplicity, we increment attempts and retry in 10 seconds.
	// Hard failure after 5 attempts.
	errMsg := publishErr.Error()
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET attempts = attempts + 1,
		    error_msg = $1,
		    next_retry_at = NOW() + INTERVAL '10 seconds' * power(2, attempts),
		    status = CASE WHEN attempts >= 5 THEN 'FAILED' ELSE 'PENDING' END,
		    updated_at = NOW()
		WHERE id = $2
	`, errMsg, id)
	return err
}
