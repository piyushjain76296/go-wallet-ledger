package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/wallet-ledger/internal/kafka"
)

type Worker struct {
	repo     Repository
	producer kafka.Producer
}

func NewWorker(repo Repository, producer kafka.Producer) *Worker {
	return &Worker{
		repo:     repo,
		producer: producer,
	}
}

// Start runs the worker continuously until the context is canceled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Outbox worker shutting down")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	events, err := w.repo.FetchPendingEvents(ctx, 50)
	if err != nil {
		slog.Error("Failed to fetch pending outbox events", "error", err)
		return
	}

	for _, ev := range events {
		// Publish to Kafka
		err := w.producer.Publish(ctx, ev.Topic, ev.Payload)
		if err != nil {
			slog.Error("Failed to publish outbox event", "id", ev.ID, "error", err)
			w.repo.MarkFailed(ctx, ev.ID, err)
			continue
		}

		// Mark as published on success
		if err := w.repo.MarkPublished(ctx, ev.ID); err != nil {
			slog.Error("Failed to mark outbox event as published", "id", ev.ID, "error", err)
		}
	}
}
