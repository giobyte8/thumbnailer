package messaging

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(ctx context.Context, body []byte) error
}

type AMQPPublisher struct {
	channel    *amqp.Channel
	exchange   string
	routingKey string
	confirms   <-chan amqp.Confirmation
}

func NewAMQPPublisher(channel *amqp.Channel, exchange string, routingKey string) *AMQPPublisher {
	return &AMQPPublisher{
		channel:    channel,
		exchange:   exchange,
		routingKey: routingKey,
		confirms:   channel.NotifyPublish(make(chan amqp.Confirmation, 1)),
	}
}

func (publisher *AMQPPublisher) Publish(ctx context.Context, body []byte) error {
	if err := publisher.channel.PublishWithContext(ctx, publisher.exchange, publisher.routingKey, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
	}); err != nil {
		return err
	}

	confirmation, ok := <-publisher.confirms
	if !ok {
		return fmt.Errorf("AMQP: publisher channel closed before confirmation")
	}
	if !confirmation.Ack {
		return fmt.Errorf("AMQP: broker rejected published message")
	}
	return nil
}
