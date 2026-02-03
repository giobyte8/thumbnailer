package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/thumbnailer/internal/models"
	"github.com/giobyte8/thumbnailer/internal/services"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

// Holds the config params for the consumer
type AMQPConfig struct {
	AMQPUri  string
	Exchange string

	ThumbsGenQueueName string
	ThumbsDelQueueName string
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
	if config.ThumbsGenQueueName == "" {
		return nil, fmt.Errorf(
			"AMQP thumbs generation queue name cannot be empty in config",
		)
	}
	if config.ThumbsDelQueueName == "" {
		return nil, fmt.Errorf(
			"AMQP thumbs delete queue name cannot be empty in config",
		)
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

	// Helper function to declare and bind a given queue
	declareAndBind := func(queueName string) error {
		_, err := c.channel.QueueDeclare(
			queueName,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return err
		}

		return c.channel.QueueBind(
			queueName,         // Queue
			queueName,         // Routing key
			c.config.Exchange, // Exchange
			false,             // No-wait
			nil,               // Arguments
		)
	}

	if err := declareAndBind(c.config.ThumbsGenQueueName); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf(
			"AMQP - Failed to declare/bind thumbs generation queue: %w",
			err,
		)
	}

	if err := declareAndBind(c.config.ThumbsDelQueueName); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf(
			"AMQP - Failed to declare/bind thumbs delete queue: %w",
			err,
		)
	}

	go c.consumeThumbsGenRequests(ctx)
	go c.consumeThumbsDelRequests(ctx)
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

func (c *AMQPConsumer) consumeThumbsGenRequests(ctx context.Context) {
	msgs, err := c.channel.Consume(
		c.config.ThumbsGenQueueName,
		"thumbnailer-gen", // Consumer tag
		false,             // Auto-acknowledge
		false,             // Exclusive
		false,             // No-local
		false,             // No-wait
		nil,               // Arguments
	)
	if err != nil {
		slog.Error(
			"AMQP - Failed to create thumbs gen queue consumer",
			"error",
			err,
		)
		return
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				slog.Info(
					"AMQP - Thumbs gen message channel closed. goroutine exiting",
				)
				return
			}

			var thumbRequest models.ThumbRequest
			err := json.Unmarshal(msg.Body, &thumbRequest)
			if err != nil {
				slog.Error(
					"AMQP - Failed to unmarshal thumbs gen message",
					"error",
					err,
					"message",
					string(msg.Body),
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error(
						"AMQP - Failed to nack thumbs gen message",
						"error",
						nackErr,
					)
				}
				continue
			}

			c.telemetry.Metrics().Increment(metrics.ThumbGenRequestReceived)

			err = c.thumbnailSvc.ProcessGenRequest(ctx, thumbRequest)
			if err != nil {
				slog.Error(
					"AMQP - Failed to process thumbnail generation request",
					"error",
					err,
					"filePath",
					thumbRequest.FilePath,
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error(
						"AMQP - Failed to nack thumbs gen message",
						"error",
						nackErr,
					)
				}
				continue
			}

			// Acknowledge the message
			if err := msg.Ack(false); err != nil {
				slog.Error(
					"AMQP - Failed to acknowledge thumbs gen message",
					"error",
					err,
				)
			}

		case <-ctx.Done():
			slog.Info(
				"AMQP - Context done signal received, " +
					"stopping thumbs gen consumption goroutine...",
			)
			return
		}
	}
}

func (c *AMQPConsumer) consumeThumbsDelRequests(ctx context.Context) {
	msgs, err := c.channel.Consume(
		c.config.ThumbsDelQueueName,
		"thumbnailer-del", // Consumer tag
		false,             // Auto-acknowledge
		false,             // Exclusive
		false,             // No-local
		false,             // No-wait
		nil,               // Arguments
	)
	if err != nil {
		slog.Error(
			"AMQP - Failed to create thumbs del queue consumer",
			"error",
			err,
		)
		return
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				slog.Info(
					"AMQP - Thumbs del message channel closed. goroutine exiting",
				)
				return
			}

			var thumbRequest models.ThumbRequest
			err := json.Unmarshal(msg.Body, &thumbRequest)
			if err != nil {
				slog.Error(
					"AMQP - Failed to unmarshal thumbs del message",
					"error",
					err,
					"message",
					string(msg.Body),
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error(
						"AMQP - Failed to nack thumbs del message",
						"error",
						nackErr,
					)
				}
				continue
			}

			c.telemetry.Metrics().Increment(metrics.ThumbDelRequestReceived)

			err = c.thumbnailSvc.ProcessDelRequest(ctx, thumbRequest)
			if err != nil {
				slog.Error(
					"AMQP - Failed to process thumbnail delete request",
					"error",
					err,
					"filePath",
					thumbRequest.FilePath,
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error(
						"AMQP - Failed to nack thumbs del message",
						"error",
						nackErr,
					)
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				slog.Error(
					"AMQP - Failed to acknowledge thumbs del message",
					"error",
					err,
				)
			}

		case <-ctx.Done():
			slog.Info(
				"AMQP - Context done signal received, " +
					"stopping thumbs del consumption goroutine...",
			)
			return
		}
	}
}
