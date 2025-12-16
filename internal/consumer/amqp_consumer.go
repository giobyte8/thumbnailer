package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/galleries/thumbnailer/internal/models"
	"github.com/giobyte8/galleries/thumbnailer/internal/services"
	"github.com/giobyte8/galleries/thumbnailer/internal/telemetry"
	"github.com/giobyte8/galleries/thumbnailer/internal/telemetry/metrics"
)

// Holds the config params for the consumer
type AMQPConfig struct {
	AMQPUri   string
	Exchange  string
	QueueName string
}

type AMQPConsumer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	config       AMQPConfig
	thumbnailSvc *services.ThumbnailsService
	telemetry    telemetry.TelemetrySvc
}

// Creates a new AMQPConsumer instance ready to connect to broker
func NewAMQPConsumer(
	config AMQPConfig,
	thumbnailSvc *services.ThumbnailsService,
	telemetry *telemetry.TelemetrySvc,
) (*AMQPConsumer, error) {

	if config.AMQPUri == "" {
		return nil, fmt.Errorf("AMQP URI cannot be empty in config")
	}
	if config.Exchange == "" {
		return nil, fmt.Errorf("AMQP exchange cannot be empty in config")
	}
	if config.QueueName == "" {
		return nil, fmt.Errorf("AMQP queue name cannot be empty in config")
	}

	return &AMQPConsumer{
		config:       config,
		thumbnailSvc: thumbnailSvc,
		telemetry:    *telemetry,
	}, nil
}

// Connects to AMQP broker, declares exchange and queue and
// starts consuming messages
func (c *AMQPConsumer) Start(ctx context.Context) error {
	slog.Debug("AMQP - Initializing AMQP Consumer")

	var err error
	c.conn, err = amqp.Dial(c.config.AMQPUri)
	if err != nil {
		return fmt.Errorf("AMQP - Connection to broker failed: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("AMQP - Failed to open channel: %w", err)
	}

	err = c.channel.ExchangeDeclare(
		c.config.Exchange,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("AMQP - Failed to declare exchange: %w", err)
	}

	_, err = c.channel.QueueDeclare(
		c.config.QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("AMQP - Failed to declare queue: %w", err)
	}

	err = c.channel.QueueBind(
		c.config.QueueName, // Queue
		c.config.QueueName, // Routing key
		c.config.Exchange,  // Exchange
		false,              // No-wait
		nil,                // Arguments
	)
	if err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("AMQP - Failed to bind queue: %w", err)
	}

	go c.consume(ctx)
	return nil
}

// Gracefully stops the AMQP consumer
func (c *AMQPConsumer) Stop() {
	slog.Info("AMQP - Stopping AMQP Consumer...")

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			slog.Error("AMQP - Failed to close channel", "error", err)
		} else {
			slog.Debug("AMQP - Channel closed")
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			slog.Error("AMQP - Failed to close connection", "error", err)
		} else {
			slog.Debug("AMQP - Connection closed")
		}
	}

	slog.Info("AMQP - AMQP Consumer stopped")
}

func (c *AMQPConsumer) consume(ctx context.Context) {
	msgs, err := c.channel.Consume(
		c.config.QueueName,
		"thumbnailer", // Consumer tag
		false,         // Auto-acknowledge
		false,         // Exclusive
		false,         // No-local
		false,         // No-wait
		nil,           // Arguments
	)
	if err != nil {
		slog.Error("AMQP - Failed to create queue consumer", "error", err)
		return
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				slog.Info("AMQP - Message channel closed. goroutine exiting")
				return
			}

			// slog.Debug("AMQP - Received message", "message", string(msg.Body))
			var fileEvt models.FileDiscoveryEvent
			err := json.Unmarshal(msg.Body, &fileEvt)
			if err != nil {
				slog.Error(
					"AMQP - Failed to unmarshal message",
					"error",
					err,
					"message",
					string(msg.Body),
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error("AMQP - Failed to nack message", "error", nackErr)
				}
				continue
			}

			c.telemetry.Metrics().Increment(
				metrics.FileEvtReceived,
				map[string]string{
					"eventType": fileEvt.EventType,
					"filePath":  fileEvt.FilePath,
				},
			)

			// TODO Verify event type (?)
			err = c.thumbnailSvc.ProcessEvent(ctx, fileEvt)
			if err != nil {
				slog.Error(
					"AMQP - Failed to process file discovery event",
					"error",
					err,
					"eventType",
					fileEvt.EventType,
					"filePath",
					fileEvt.FilePath,
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error("AMQP - Failed to nack message", "error", nackErr)
				}
				continue
			}

			// Acknowledge the message
			if err := msg.Ack(false); err != nil {
				slog.Error("AMQP - Failed to acknowledge message", "error", err)
			}

		case <-ctx.Done():
			slog.Info(
				"AMQP - Context done signal received, stopping consumption goroutine...",
			)
			return
		}
	}
}
