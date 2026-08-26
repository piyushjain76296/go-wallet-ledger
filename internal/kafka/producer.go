package kafka

import (
	"context"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Close()
}

type franzProducer struct {
	client *kgo.Client
}

func NewProducer(broker string) (Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.AllowAutoTopicCreation(), // Development only
	)
	if err != nil {
		return nil, err
	}

	if err := cl.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &franzProducer{client: cl}, nil
}

func (p *franzProducer) Publish(ctx context.Context, topic string, payload []byte) error {
	record := &kgo.Record{
		Topic: topic,
		Value: payload,
	}
	
	// Synchronous publish (Wait) to guarantee delivery before updating Outbox
	results := p.client.ProduceSync(ctx, record)
	if results.FirstErr() != nil {
		slog.Error("Failed to publish message to Kafka", "topic", topic, "error", results.FirstErr())
		return results.FirstErr()
	}

	return nil
}

func (p *franzProducer) Close() {
	p.client.Close()
}
