package consumer

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueConsumer struct {
	channel   *amqp.Channel
	queueName string
}

// Reusable function to consume messages from a given queue with a provided
// callback for message processing.
func NewQueueConsumer(
	channel *amqp.Channel,
	queueName string,
) *QueueConsumer {

	return &QueueConsumer{
		channel:   channel,
		queueName: queueName,
	}
}

func (c *QueueConsumer) Start(
	ctx context.Context,
	onMessage func(message amqp.Delivery) error,
) error {
	consumer_name := "thumbs-consumer:" + c.queueName

	messages, err := c.channel.Consume(
		c.queueName,
		consumer_name,
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("consumer startup failure: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info(
				"AMQP: Context done signal received. Stopping consumer",
				"consumer", consumer_name,
			)
			return nil

		case msg, ok := <-messages:
			if !ok {
				slog.Warn(
					"AMQP: Channel closed. Exiting consumer",
					"consumer", consumer_name,
				)
				return nil
			}

			// Invoke callback for message
			err := onMessage(msg)
			if err != nil {
				slog.Error(
					"AMQP: Error processing message",
					"consumer", consumer_name,
					"error", err,
				)

				// Nacknowledge the message
				if nackErr := msg.Nack(false, false); nackErr != nil {
					slog.Error(
						"AMQP: Failed to nack message",
						"consumer", consumer_name,
						"error", nackErr,
					)
				}
				continue
			}

			// Acknowledge the message
			if err := msg.Ack(false); err != nil {
				slog.Error(
					"AMQP: Failed to acknowledge message",
					"consumer", consumer_name,
					"error", err,
				)
			}
		}
	}
}
