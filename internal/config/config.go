package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	LogLevel        slog.Level
	Amqp            AmqpConfig
	RootDirs        RootDirsConfig
	ThumbnailWidths []int
	Otel            OtelConfig
}

type AmqpConfig struct {
	Host  string
	Port  string
	User  string
	Pass  string
	VHost string

	ExchangeName       string
	ThumbsGenQueueName string
	ThumbsDelQueueName string
}

func (c AmqpConfig) Uri() string {
	vhost := url.PathEscape(c.VHost)
	if vhost == "" {
		vhost = "%2F"
	}

	return fmt.Sprintf(
		"amqp://%s@%s/%s",
		url.UserPassword(c.User, c.Pass),
		net.JoinHostPort(c.Host, c.Port),
		vhost,
	)
}

type RootDirsConfig struct {
	Originals  string
	Thumbnails string
}

type OtelConfig struct {
	Enabled               bool
	CollectorGrpcEndpoint string
}

var (
	instanceOnce sync.Once
	instance     *AppConfig
	instanceErr  error
)

// --- Public API --- --- --- --- --- --- --- --- --- --- --- --- --- --- --- -

func LogLevel() slog.Level {
	return AppCfg().LogLevel
}

func Amqp() AmqpConfig {
	return AppCfg().Amqp
}

func RootDirs() RootDirsConfig {
	return AppCfg().RootDirs
}

func ThumbWidthsPx() []int {
	return AppCfg().ThumbnailWidths
}

func Otel() OtelConfig {
	return AppCfg().Otel
}

// --- Initializers --- --- --- --- --- --- --- --- --- --- --- --- --- --- ---

// Exposes a singleton AppConfig instance loaded from environment
// variables (or .env file)
func AppCfg() *AppConfig {
	instanceOnce.Do(func() {
		instanceErr = loadDotEnv()
		if instanceErr != nil {
			return
		}

		instance, instanceErr = newAppConfig()
	})

	if instanceErr != nil {
		panic(instanceErr)
	}

	return instance
}

func newAppConfig() (*AppConfig, error) {
	rootDirsCfg, err := newRootDirsConfig()
	if err != nil {
		return nil, err
	}

	thumbnailWidths, err := parseThumbnailWidths(os.Getenv("THUMBNAIL_WIDTHS_PX"))
	if err != nil {
		return nil, err
	}

	return &AppConfig{
		LogLevel:        parseLogLevel(os.Getenv("LOG_LEVEL")),
		Amqp:            newAmqpConfig(),
		RootDirs:        rootDirsCfg,
		ThumbnailWidths: thumbnailWidths,
		Otel:            newOtelConfig(),
	}, nil
}

func newAmqpConfig() AmqpConfig {
	return AmqpConfig{
		Host:  os.Getenv("RABBITMQ_HOST"),
		Port:  os.Getenv("RABBITMQ_PORT"),
		User:  os.Getenv("RABBITMQ_USER"),
		Pass:  os.Getenv("RABBITMQ_PASS"),
		VHost: os.Getenv("RABBITMQ_VHOST"),

		ExchangeName:       os.Getenv("AMQP_EXCHANGE"),
		ThumbsGenQueueName: os.Getenv("AMQP_QUEUE_THUMB_GEN_REQUESTS"),
		ThumbsDelQueueName: os.Getenv("AMQP_QUEUE_THUMB_DEL_REQUESTS"),
	}
}

func newRootDirsConfig() (RootDirsConfig, error) {
	rootDirsCfg := RootDirsConfig{
		Originals:  os.Getenv("DIR_ORIGINALS_ROOT"),
		Thumbnails: os.Getenv("DIR_THUMBNAILS_ROOT"),
	}
	if rootDirsCfg.Originals == "" || rootDirsCfg.Thumbnails == "" {
		return RootDirsConfig{}, fmt.Errorf(
			"missing required environment variables for thumbnails service: "+
				"DIR_ORIGINALS_ROOT=%q DIR_THUMBNAILS_ROOT=%q",
			rootDirsCfg.Originals,
			rootDirsCfg.Thumbnails,
		)
	}

	return rootDirsCfg, nil
}

func newOtelConfig() OtelConfig {
	return OtelConfig{
		Enabled:               strings.EqualFold(os.Getenv("OTEL_ENABLED"), "true"),
		CollectorGrpcEndpoint: os.Getenv("OTEL_COLLECTOR_GRPC_ENDPOINT"),
	}
}

// --- Parsers --- --- --- --- --- --- --- --- --- --- --- --- --- --- --- ---

func loadDotEnv() error {
	if _, err := os.Stat(".env"); err != nil {
		if os.IsNotExist(err) {
			slog.Debug("No .env file found, using environment variables directly.")
			return nil
		}

		return fmt.Errorf("failed to stat .env file: %w", err)
	}

	if err := godotenv.Load(".env"); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	return nil
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseThumbnailWidths(value string) ([]int, error) {
	if value == "" {
		return nil, fmt.Errorf("THUMBNAIL_WIDTHS_PX is required")
	}

	widthValues := strings.Split(value, ",")
	widths := make([]int, 0, len(widthValues))

	for _, rawWidth := range widthValues {
		width, err := strconv.Atoi(strings.TrimSpace(rawWidth))
		if err != nil {
			return nil, fmt.Errorf(
				"invalid thumbnail width in THUMBNAIL_WIDTHS_PX %q: %w",
				rawWidth,
				err,
			)
		}

		if width <= 0 {
			return nil, fmt.Errorf(
				"thumbnail width must be a positive integer: %d",
				width,
			)
		}

		widths = append(widths, width)
	}

	return widths, nil
}
