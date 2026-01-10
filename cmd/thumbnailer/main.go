package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/giobyte8/thumbnailer/internal/consumer"
	"github.com/giobyte8/thumbnailer/internal/services"
	"github.com/joho/godotenv"

	"github.com/giobyte8/thumbnailer/internal/telemetry"
	thumbsgen "github.com/giobyte8/thumbnailer/internal/thumbs_gen"
)

func setupLogging() {
	var log_level slog.Level
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG", "debug":
		log_level = slog.LevelDebug
	case "WARN", "warn":
		log_level = slog.LevelWarn
	case "ERROR", "error":
		log_level = slog.LevelError
	default:
		log_level = slog.LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     log_level,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {

			// Format time to show only the time (HH:MM:SS)
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("15:04:05"))
			}

			return a
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOpts))
	slog.SetDefault(logger)
}

func loadEnv() {
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		slog.Warn("No .env file found, using environment variables directly.")
		return
	}

	err := godotenv.Load(".env")
	if err != nil {
		slog.Error("Error loading .env file", "error", err)
		os.Exit(1)
	}
}

func prepareAMQPUri() string {
	rb_host := os.Getenv("RABBITMQ_HOST")
	rb_port := os.Getenv("RABBITMQ_PORT")
	rb_user := os.Getenv("RABBITMQ_USER")
	rb_pass := os.Getenv("RABBITMQ_PASS")

	return fmt.Sprintf(
		"amqp://%s:%s@%s:%s/",
		rb_user,
		rb_pass,
		rb_host,
		rb_port,
	)
}

func prepareAMQPConsumer(
	telemetry *telemetry.TelemetrySvc,
) (consumer.MessageConsumer, error) {
	var amqpCfg consumer.AMQPConfig
	amqpCfg.AMQPUri = prepareAMQPUri()
	amqpCfg.Exchange = os.Getenv("AMQP_EXCHANGE")
	amqpCfg.ThumbsGenQueueName = os.Getenv("AMQP_QUEUE_THUMB_GEN_REQUESTS")
	amqpCfg.ThumbsDelQueueName = os.Getenv("AMQP_QUEUE_THUMB_DEL_REQUESTS")

	return consumer.NewAMQPConsumer(
		amqpCfg,
		prepareThumbsService(telemetry),
		telemetry,
	)
}

func prepareThumbsService(telemetry *telemetry.TelemetrySvc) *services.ThumbnailsService {
	thumbsConfig := services.ThumbnailsConfig{
		DirOriginalsRoot:  os.Getenv("DIR_ORIGINALS_ROOT"),
		DirThumbnailsRoot: os.Getenv("DIR_THUMBNAILS_ROOT"),
	}

	if thumbsConfig.DirOriginalsRoot == "" || thumbsConfig.DirThumbnailsRoot == "" {
		slog.Error(
			"Missing required environment variables for thumbnails service",
			"DIR_ORIGINALS_ROOT", thumbsConfig.DirOriginalsRoot,
			"DIR_THUMBNAILS_ROOT", thumbsConfig.DirThumbnailsRoot,
		)
		os.Exit(1)
	}

	widthsStr := os.Getenv("THUMBNAIL_WIDTHS_PX")

	// Default widths if not provided
	if widthsStr == "" {
		slog.Warn(
			"THUMBNAIL_WIDTHS_PX is not set. Using defaults.",
			"default",
			"256,512,1024",
		)
		widthsStr = "256,512,1024"
	}

	// Parse widths
	widthStrs := strings.Split(widthsStr, ",")
	for _, ws := range widthStrs {

		// Trim spaces in case of "256, 512"
		width, err := strconv.Atoi(strings.TrimSpace(ws))
		if err != nil {
			slog.Error(
				"Invalid thumbnail width in THUMBNAIL_WIDTHS_PX",
				"width",
				ws,
				"error",
				err,
			)
			os.Exit(1)
		}

		if width <= 0 {
			slog.Error(
				"Thumbnail width must be a positive integer", "width", width,
			)
			os.Exit(1)
		}

		thumbsConfig.ThumbnailWidths = append(
			thumbsConfig.ThumbnailWidths,
			width,
		)
	}

	thumbsGenerator := thumbsgen.NewLilliputThumbsGenerator(telemetry)
	return services.NewThumbnailsService(thumbsConfig, thumbsGenerator)
}

func main() {
	loadEnv()
	setupLogging()

	slog.Info("Starting Thumbnailer service...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Init telemetry services
	telemetry, err := telemetry.NewTelemetrySvc(ctx)
	if err != nil {
		slog.Error("Failed to initialize Telemetry services", "error", err)
		os.Exit(1)
	}

	amqpConsumer, err := prepareAMQPConsumer(telemetry)
	if err != nil {
		slog.Error("Failed to create AMQP consumer", "error", err)
		os.Exit(1)
	}

	if err := amqpConsumer.Start(ctx); err != nil {
		slog.Error("Failed to start AMQP consumer", "error", err)
		os.Exit(1)
	}
	slog.Info("Thumbnailer service is running. Press Ctrl+C to stop.")

	// Graceful shutdown (listen for OS signals)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sigChan:
		slog.Info("Received OS signal, shutting down...", "signal", s.String())
	case <-ctx.Done():
		slog.Info(
			"Parent context cancelled, shutting down...",
			"reason",
			ctx.Err(),
		)
	}

	// --- --- --- --- --- --- --- --- --- --- --- ---
	// Perform graceful shutdown operations
	// before cancelling context

	amqpConsumer.Stop()
	if err := telemetry.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown telemetry services", "error", err)
	}

	// Trigger context cancellation
	cancel()
	slog.Info("Thumbnailer service exited gracefully.")
}
