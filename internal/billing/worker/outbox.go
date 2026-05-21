// Package worker holds background loops. For billing-service that's currently
// just the outbox publisher.
package worker

import (
	"context"
	"time"

	"github.com/farid/billing-service/internal/billing/repository"
	"github.com/farid/billing-service/pkg/logger"
	"github.com/farid/billing-service/pkg/rabbit"
)

type OutboxPublisher struct {
	repo      repository.OutboxRepository
	publisher *rabbit.Publisher
	interval  time.Duration
	batch     int
}

func NewOutboxPublisher(repo repository.OutboxRepository, p *rabbit.Publisher) *OutboxPublisher {
	return &OutboxPublisher{repo: repo, publisher: p, interval: time.Second, batch: 200}
}

// Run continuously drains outbox_event into RabbitMQ. Same protocol as
// reservation-service: at-least-once delivery, consumer-side dedup expected.
func (w *OutboxPublisher) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *OutboxPublisher) tick(ctx context.Context) {
	rows, err := w.repo.FetchUnpublished(ctx, w.batch)
	if err != nil {
		logger.Error(ctx, "billing outbox: fetch failed",
			map[string]interface{}{logger.ErrorKey: err.Error()})
		return
	}
	if len(rows) == 0 {
		return
	}
	var published []int64
	for _, r := range rows {
		if err := w.publisher.Publish(ctx, r.EventType, r.Payload); err != nil {
			logger.Error(ctx, "billing outbox: publish failed",
				map[string]interface{}{
					"id":            r.ID,
					"event_type":    r.EventType,
					logger.ErrorKey: err.Error(),
				})
			break
		}
		published = append(published, r.ID)
	}
	if err := w.repo.MarkPublished(ctx, published); err != nil {
		logger.Error(ctx, "billing outbox: mark failed",
			map[string]interface{}{logger.ErrorKey: err.Error()})
	}
}
