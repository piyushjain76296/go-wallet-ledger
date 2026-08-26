package idempotency

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrKeyInProgress = errors.New("request is currently in progress")
)

type Status string

const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
)

type KeyRecord struct {
	Key             string
	Status          Status
	RequestPayload  json.RawMessage
	ResponsePayload json.RawMessage
	ResponseStatus  int
}

type Repository interface {
	TryAcquire(ctx context.Context, key string, requestPayload []byte) (*KeyRecord, error)
	Complete(ctx context.Context, key string, responsePayload []byte, statusCode int) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) TryAcquire(ctx context.Context, key string, requestPayload []byte) (*KeyRecord, error) {
	// 1. Try to insert
	_, err := r.db.Exec(ctx, `
		INSERT INTO idempotency_keys (key, status, request_payload, locked_at)
		VALUES ($1, $2, $3, NOW())
	`, key, StatusInProgress, requestPayload)

	if err == nil {
		// Successfully inserted, it's ours to process
		return nil, nil 
	}

	// 2. If insert fails (PK conflict), fetch the existing record.
	var rec KeyRecord
	var responsePayload []byte
	var responseStatus *int
	err = r.db.QueryRow(ctx, `
		SELECT key, status, request_payload, response_payload, response_status
		FROM idempotency_keys
		WHERE key = $1
	`, key).Scan(&rec.Key, &rec.Status, &rec.RequestPayload, &responsePayload, &responseStatus)
	
	if err != nil {
		return nil, err
	}

	if responsePayload != nil {
		rec.ResponsePayload = responsePayload
	}
	if responseStatus != nil {
		rec.ResponseStatus = *responseStatus
	}

	if rec.Status == StatusInProgress {
		return nil, ErrKeyInProgress
	}

	// It's completed. Return the previous result.
	return &rec, nil
}

func (r *postgresRepository) Complete(ctx context.Context, key string, responsePayload []byte, statusCode int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = $1, response_payload = $2, response_status = $3
		WHERE key = $4
	`, StatusCompleted, responsePayload, statusCode, key)
	return err
}
