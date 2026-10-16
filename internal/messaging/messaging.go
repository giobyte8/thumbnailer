package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/thumbnailer/internal/config"
	"github.com/giobyte8/thumbnailer/internal/services"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
)

type Messaging struct {
	resources    *AMQPResources
	thumbnailSvc *services.ThumbnailsService
	telemetry    telemetry.TelemetrySvc
}

func NewMessaging(
	thumbnailSvc *services.ThumbnailsService,
	telemetry *telemetry.TelemetrySvc,
) *Messaging {
	return &Messaging{
		resources:    NewAMQPResources(),
		thumbnailSvc: thumbnailSvc,
		telemetry:    *telemetry,
	}
}

func (messaging *Messaging) Start(ctx context.Context) error {
	for attempt := 1; attempt <= 3; attempt++ {
		if err := messaging.resources.ConnectAndSetup(); err != nil {
			if attempt == 3 {
				return fmt.Errorf("AMQP: connection/setup max attempts reached: %w", err)
			}
			slog.Warn("AMQP: connection/setup attempt failed", "attempt", attempt, "error", err)

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil
			}
			continue
		}

		cfg := config.Amqp()
		producer := NewAMQPPublisher(
			messaging.resources.ProducerChannel(),
			cfg.ExchangeName,
			cfg.ThumbsGenOverAllowedDimensionQueueName)
		consumer := NewAMQPConsumer(
			messaging.resources.ConsumerChannel(),
			messaging.thumbnailSvc,
			&messaging.telemetry,
			producer)
		consumer.Start(ctx)

		connectionClosed := make(chan *amqp.Error, 1)
		consumerChannelClosed := make(chan *amqp.Error, 1)
		producerChannelClosed := make(chan *amqp.Error, 1)
		messaging.resources.Connection().NotifyClose(connectionClosed)
		messaging.resources.ConsumerChannel().NotifyClose(consumerChannelClosed)
		messaging.resources.ProducerChannel().NotifyClose(producerChannelClosed)

		select {
		case <-ctx.Done():
			messaging.Stop()
			return nil
		case err := <-connectionClosed:
			messaging.resources.Stop()
			slog.Warn("AMQP: connection closed, reconnecting", "error", err)
			attempt = 0
		case err := <-consumerChannelClosed:
			messaging.resources.Stop()
			slog.Warn("AMQP: consumer channel closed, reconnecting", "error", err)
			attempt = 0
		case err := <-producerChannelClosed:
			messaging.resources.Stop()
			slog.Warn("AMQP: producer channel closed, reconnecting", "error", err)
			attempt = 0
		}
	}
	return nil
}

func (messaging *Messaging) Stop() { messaging.resources.Stop() }
