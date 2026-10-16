package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/giobyte8/thumbnailer/internal/config"
	"github.com/giobyte8/thumbnailer/internal/models"
	"github.com/giobyte8/thumbnailer/internal/services"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	thumbsgen "github.com/giobyte8/thumbnailer/internal/thumbs_gen"
)

type AMQPConsumer struct {
	channel      *amqp.Channel
	thumbnailSvc *services.ThumbnailsService
	telemetry    telemetry.TelemetrySvc
	producer     Publisher
	ctx          context.Context
}

func NewAMQPConsumer(channel *amqp.Channel, thumbnailSvc *services.ThumbnailsService, telemetry *telemetry.TelemetrySvc, producer Publisher) *AMQPConsumer {
	return &AMQPConsumer{channel: channel, thumbnailSvc: thumbnailSvc, telemetry: *telemetry, producer: producer}
}

func (consumer *AMQPConsumer) Start(ctx context.Context) {
	consumer.ctx = ctx
	cfg := config.Amqp()
	thGenConsumer := NewQueueConsumer(consumer.channel, cfg.ThumbsGenQueueName)
	thDelConsumer := NewQueueConsumer(consumer.channel, cfg.ThumbsDelQueueName)

	go func() {
		if err := thGenConsumer.Start(ctx, consumer.processGenMessage); err != nil {
			slog.Error("AMQP: thumbGen consumer failed", "error", err)
		}
	}()
	go func() {
		if err := thDelConsumer.Start(ctx, consumer.processDelMessage); err != nil {
			slog.Error("AMQP: thumbDel consumer failed", "error", err)
		}
	}()
}

func (consumer *AMQPConsumer) processGenMessage(message amqp.Delivery) error {
	var thumbRequest models.ThumbRequest
	if err := json.Unmarshal(message.Body, &thumbRequest); err != nil {
		return err
	}

	err := consumer.thumbnailSvc.ProcessGenRequest(consumer.ctx, thumbRequest)
	var dimensionErr *thumbsgen.MaxImgDimensionExceededError
	if !errors.As(err, &dimensionErr) {
		return err
	}

	slog.Warn("Image dimension exceeds allowed maximum, rerouting request", "filePath", thumbRequest.FilePath, "width", dimensionErr.Width, "height", dimensionErr.Height, "maxAllowedDimension", dimensionErr.MaxAllowedDimension)
	return consumer.producer.Publish(consumer.ctx, message.Body)
}

func (consumer *AMQPConsumer) processDelMessage(message amqp.Delivery) error {
	var thumbRequest models.ThumbRequest
	if err := json.Unmarshal(message.Body, &thumbRequest); err != nil {
		return err
	}
	return consumer.thumbnailSvc.ProcessDelRequest(consumer.ctx, thumbRequest)
}
