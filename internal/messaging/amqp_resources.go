package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/thumbnailer/internal/config"
)

type AMQPResources struct {
	conn            *amqp.Connection
	consumerChannel *amqp.Channel
	producerChannel *amqp.Channel
}

func NewAMQPResources() *AMQPResources { return &AMQPResources{} }

func (resources *AMQPResources) ConnectAndSetup() error {
	cfg := config.Amqp()
	conn, err := amqp.Dial(cfg.Uri())
	if err != nil {
		return fmt.Errorf("AMQP: connection to broker failed: %w", err)
	}
	consumerChannel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("AMQP: failed to open consumer channel: %w", err)
	}
	producerChannel, err := conn.Channel()
	if err != nil {
		consumerChannel.Close()
		conn.Close()
		return fmt.Errorf("AMQP: failed to open producer channel: %w", err)
	}
	if err := ensureBrokerResources(consumerChannel, cfg); err != nil {
		producerChannel.Close()
		consumerChannel.Close()
		conn.Close()
		return err
	}
	if err := consumerChannel.Qos(10, 0, false); err != nil {
		producerChannel.Close()
		consumerChannel.Close()
		conn.Close()
		return fmt.Errorf("AMQP: failed to set consumer QoS: %w", err)
	}
	if err := producerChannel.Confirm(false); err != nil {
		producerChannel.Close()
		consumerChannel.Close()
		conn.Close()
		return fmt.Errorf("AMQP: failed to enable publisher confirms: %w", err)
	}
	resources.conn, resources.consumerChannel, resources.producerChannel = conn, consumerChannel, producerChannel
	return nil
}

func (resources *AMQPResources) ConsumerChannel() *amqp.Channel { return resources.consumerChannel }
func (resources *AMQPResources) ProducerChannel() *amqp.Channel { return resources.producerChannel }
func (resources *AMQPResources) Connection() *amqp.Connection   { return resources.conn }

func (resources *AMQPResources) Stop() {
	if resources.producerChannel != nil {
		_ = resources.producerChannel.Close()
	}
	if resources.consumerChannel != nil {
		_ = resources.consumerChannel.Close()
	}
	if resources.conn != nil {
		_ = resources.conn.Close()
	}
}

func ensureBrokerResources(channel *amqp.Channel, cfg config.AmqpConfig) error {
	if err := channel.ExchangeDeclare(cfg.ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("AMQP: failed to declare exchange: %w", err)
	}
	for _, queueName := range []string{cfg.ThumbsGenQueueName, cfg.ThumbsDelQueueName, cfg.ThumbsGenOverAllowedDimensionQueueName} {
		if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("AMQP: failed to declare queue %s: %w", queueName, err)
		}
		if err := channel.QueueBind(queueName, queueName, cfg.ExchangeName, false, nil); err != nil {
			return fmt.Errorf("AMQP: failed to bind queue %s: %w", queueName, err)
		}
	}
	return nil
}
