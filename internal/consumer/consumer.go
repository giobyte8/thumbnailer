package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/thumbnailer/internal/config"
	"github.com/giobyte8/thumbnailer/internal/models"
	"github.com/giobyte8/thumbnailer/internal/services"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
)

type AMQPConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel

	thumbnailSvc *services.ThumbnailsService
	telemetry    telemetry.TelemetrySvc
}

// Creates a new AMQPConsumer instance ready to connect to broker
func NewAMQPConsumer(
	thumbnailSvc *services.ThumbnailsService,
	telemetry *telemetry.TelemetrySvc,
) *AMQPConsumer {
	return &AMQPConsumer{
		thumbnailSvc: thumbnailSvc,
		telemetry:    *telemetry,
	}
}

// Connects to AMQP broker, declares exchange and queue and
// starts consuming messages
func (consumer *AMQPConsumer) Start(ctx context.Context) error {
	slog.Debug("AMQP: Starting consumer")

	maxRetries := 3
	retryDelay := 5 // seconds

	for attempt := 1; attempt <= maxRetries; attempt++ {

		// Attempt to connect and setup
		err := consumer.connectAndSetup()

		// Successful connection
		if err == nil {
			if attempt > 1 {
				slog.Info(
					"AMQP: Connection/setup successful",
					"attempt", attempt,
				)
			}

			// Subscribe to connection close events
			connCloseChan := make(chan *amqp.Error, 1)
			consumer.conn.NotifyClose(connCloseChan)

			// Subscribe to chanel close events
			chanCloseChan := make(chan *amqp.Error, 1)
			consumer.channel.NotifyClose(chanCloseChan)

			// Consume from each queue
			consumer.consume(ctx)

			select {

			// Handle connection close events
			case amqpErr := <-connCloseChan:
				slog.Warn(
					"AMQP: Connection closed, will attempt to reconnect",
					"error", amqpErr,
				)

				attempt = 0 // reset attempts counter
				continue

			// Handle channel close events
			case amqpErr := <-chanCloseChan:
				slog.Warn(
					"AMQP: Channel closed, will attempt to reconnect",
					"error", amqpErr,
				)

				attempt = 0 // reset attempts counter
				continue

			// Handle context cancellation for graceful shutdown
			case <-ctx.Done():
				slog.Debug("AMQP: Shutting down consumer")
				consumer.Stop()
				return nil
			}
		}

		// Connection attempt failed, log and retry
		slog.Warn(
			"AMQP: Connection/setup attempt failed",
			"attempt", attempt,
			"error", err)
		if attempt == maxRetries {
			return fmt.Errorf("AMQP: Connection/setup max attempts reached")
		}

		// Wait before retrying
		slog.Warn(
			"AMQP: Retrying connection/setup",
			"delay seconds", retryDelay,
		)
		select {
		case <-time.After(time.Duration(retryDelay) * time.Second):
			continue
		case <-ctx.Done():
			slog.Info("AMQP: Context cancelled during retry wait")
			return nil
		}
	}

	return nil
}

// Gracefully stops the AMQP consumer
func (consumer *AMQPConsumer) Stop() {
	slog.Info("AMQP - Stopping AMQP Consumer...")

	if consumer.channel != nil {
		if err := consumer.channel.Close(); err != nil {
			slog.Error("AMQP - Failed to close channel", "error", err)
		} else {
			slog.Debug("AMQP - Channel closed")
		}
	}

	if consumer.conn != nil {
		if err := consumer.conn.Close(); err != nil {
			slog.Error("AMQP - Failed to close connection", "error", err)
		} else {
			slog.Debug("AMQP - Connection closed")
		}
	}

	slog.Info("AMQP - AMQP Consumer stopped")
}

func (consumer *AMQPConsumer) consume(ctx context.Context) {
	thGenQueueName := config.Amqp().ThumbsGenQueueName
	thDelQueueName := config.Amqp().ThumbsDelQueueName

	// Create a consumer instance for each queue
	thGenConsumer := NewQueueConsumer(consumer.channel, thGenQueueName)
	thDelConsumer := NewQueueConsumer(consumer.channel, thDelQueueName)

	// Run queue consumers concurrently in separate goroutines.
	// Each logs errors if any, no need to handle since  reconnect is
	// triggered by closeChan on connection drop.

	// Consume thumbnail generation requests concurrently
	go func() {
		if err := thGenConsumer.Start(ctx, func(message amqp.Delivery) error {
			var thumbRequest models.ThumbRequest
			err := json.Unmarshal(message.Body, &thumbRequest)
			if err != nil {
				return err
			}

			return consumer.thumbnailSvc.ProcessGenRequest(ctx, thumbRequest)
		}); err != nil {
			slog.Error("AMQP: thumbGen consumer failed", "error", err)
		}
	}()

	// Consume thumbnail deletion requests concurrently
	go func() {
		if err := thDelConsumer.Start(ctx, func(message amqp.Delivery) error {
			var thumbRequest models.ThumbRequest
			err := json.Unmarshal(message.Body, &thumbRequest)
			if err != nil {
				return err
			}

			return consumer.thumbnailSvc.ProcessDelRequest(ctx, thumbRequest)
		}); err != nil {
			slog.Error("AMQP: thumbDel consumer failed", "error", err)
		}
	}()
}

func (consumer *AMQPConsumer) connectAndSetup() error {
	var err error

	// Establish connection
	consumer.conn, err = amqp.Dial(config.Amqp().Uri())
	if err != nil {
		return fmt.Errorf("AMQP: Connection to broker failed: %w", err)
	}

	// Open channel
	consumer.channel, err = consumer.conn.Channel()
	if err != nil {
		consumer.conn.Close()
		return fmt.Errorf("AMQP: Failed to open channel: %w", err)
	}

	// Init broker resources (exchange, queues, bindings)
	if err := consumer.ensureBrokerResources(config.Amqp()); err != nil {
		consumer.channel.Close()
		consumer.conn.Close()
		return err
	}

	// Setup QoS to prefetch 'x' messages at a time
	if err := consumer.channel.Qos(10, 0, false); err != nil {
		consumer.channel.Close()
		consumer.conn.Close()
		return fmt.Errorf("AMQP: Failed to set consumer QoS: %w", err)
	}

	return nil
}

func (consumer *AMQPConsumer) ensureBrokerResources(cfg config.AmqpConfig) error {
	if err := consumer.declareExchange(cfg.ExchangeName); err != nil {
		consumer.channel.Close()
		consumer.conn.Close()

		return fmt.Errorf("AMQP: Failed to declare exchange: %w", err)
	}

	// Prepare array with queue names to declare and bind
	queueNames := []string{
		cfg.ThumbsGenQueueName,
		cfg.ThumbsDelQueueName,
	}

	// Declare and bind each queue, exiting on error
	for _, queueName := range queueNames {
		if err := consumer.declareAndBindQueue(cfg.ExchangeName, queueName); err != nil {
			consumer.channel.Close()
			consumer.conn.Close()

			return fmt.Errorf(
				"AMQP: Failed to declare and bind queue: %s: %w",
				queueName,
				err,
			)
		}
	}

	return nil
}

// Ensures existence of a durable direct exchange in the broker
// with the specified name
func (consumer *AMQPConsumer) declareExchange(exchangeName string) error {
	return consumer.channel.ExchangeDeclare(
		exchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
}

// Ensures existence of specified queue in the broker and binds it to the
// exchange using the queue name as the routing key.
func (consumer *AMQPConsumer) declareAndBindQueue(
	exchangeName string,
	queueName string,
) error {

	_, err := consumer.channel.QueueDeclare(
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

	return consumer.channel.QueueBind(
		queueName,    // Queue
		queueName,    // Routing key
		exchangeName, // Exchange
		false,        // No-wait
		nil,          // Arguments
	)
}
