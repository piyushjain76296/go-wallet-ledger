package outbox

import (
	"encoding/json"
	"time"
)

type EventStatus string

const (
	StatusPending   EventStatus = "PENDING"
	StatusPublished EventStatus = "PUBLISHED"
	StatusFailed    EventStatus = "FAILED"
)

type Event struct {
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	Payload     json.RawMessage `json:"payload"`
	Status      EventStatus     `json:"status"`
	Attempts    int             `json:"attempts"`
	NextRetryAt time.Time       `json:"next_retry_at"`
	ErrorMsg    string          `json:"error_msg"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
