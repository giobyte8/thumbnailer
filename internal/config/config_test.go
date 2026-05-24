package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestConfigLoadsDotEnvAndParsesValues(t *testing.T) {
	tmpDir := t.TempDir()
	writeDotEnv(t, tmpDir, `
LOG_LEVEL=WARN
RABBITMQ_HOST=broker.local
RABBITMQ_PORT=5673
RABBITMQ_USER=guest
RABBITMQ_PASS=secret
RABBITMQ_VHOST=media
AMQP_EXCHANGE=thumbs
AMQP_QUEUE_THUMB_GEN_REQUESTS=thumbs-gen
AMQP_QUEUE_THUMB_DEL_REQUESTS=thumbs-del
DIR_ORIGINALS_ROOT=/data/originals
DIR_THUMBNAILS_ROOT=/data/thumbs
THUMBNAIL_WIDTHS_PX="128, 256,512"
OTEL_ENABLED=true
OTEL_COLLECTOR_GRPC_ENDPOINT=collector.local:4317
`)
	chdir(t, tmpDir)

	resetForTests()
	cfg := AppCfg()

	if got := cfg.LogLevel; got != slog.LevelWarn {
		t.Fatalf("LogLevel = %v, want %v", got, slog.LevelWarn)
	}

	amqpCfg := cfg.Amqp
	if amqpCfg.Host != "broker.local" || amqpCfg.Port != "5673" {
		t.Fatalf("AMQP host/port = %+v, want broker.local:5673", amqpCfg)
	}
	if amqpCfg.User != "guest" || amqpCfg.Pass != "secret" {
		t.Fatalf("AMQP credentials = %+v, want guest/secret", amqpCfg)
	}
	if amqpCfg.VHost != "media" || amqpCfg.ExchangeName != "thumbs" {
		t.Fatalf("AMQP vhost/exchange = %+v, want media/thumbs", amqpCfg)
	}
	if amqpCfg.ThumbsGenQueueName != "thumbs-gen" || amqpCfg.ThumbsDelQueueName != "thumbs-del" {
		t.Fatalf("AMQP queues = %+v, want thumbs-gen/thumbs-del", amqpCfg)
	}
	if got := amqpCfg.Uri(); got != "amqp://guest:secret@broker.local:5673/media" {
		t.Fatalf("AMQP Uri = %q, want %q", got, "amqp://guest:secret@broker.local:5673/media")
	}

	rootDirs := cfg.RootDirs
	if rootDirs.Originals != "/data/originals" || rootDirs.Thumbnails != "/data/thumbs" {
		t.Fatalf("RootDirs = %+v, want /data/originals and /data/thumbs", rootDirs)
	}

	if got := cfg.ThumbnailWidths; !reflect.DeepEqual(got, []int{128, 256, 512}) {
		t.Fatalf("ThumbnailWidths = %v, want [128 256 512]", got)
	}

	otelCfg := cfg.Otel
	if !otelCfg.Enabled || otelCfg.CollectorGrpcEndpoint != "collector.local:4317" {
		t.Fatalf("Otel = %+v, want enabled with collector.local:4317", otelCfg)
	}
}

func TestAMQPURIEscapesDefaultVHost(t *testing.T) {
	cfg := AmqpConfig{
		Host: "broker.local",
		Port: "5672",
		User: "guest",
		Pass: "secret",
	}

	if got := cfg.Uri(); got != "amqp://guest:secret@broker.local:5672/%2F" {
		t.Fatalf("AMQP Uri = %q, want %q", got, "amqp://guest:secret@broker.local:5672/%2F")
	}
}

func TestConfigRejectsMissingThumbnailWidths(t *testing.T) {
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	t.Setenv("LOG_LEVEL", "")
	t.Setenv("RABBITMQ_HOST", "localhost")
	t.Setenv("RABBITMQ_PORT", "5672")
	t.Setenv("RABBITMQ_USER", "user")
	t.Setenv("RABBITMQ_PASS", "pass")
	t.Setenv("RABBITMQ_VHOST", "/")
	t.Setenv("AMQP_EXCHANGE", "exchange")
	t.Setenv("AMQP_QUEUE_THUMB_GEN_REQUESTS", "gen")
	t.Setenv("AMQP_QUEUE_THUMB_DEL_REQUESTS", "del")
	t.Setenv("DIR_ORIGINALS_ROOT", "/orig")
	t.Setenv("DIR_THUMBNAILS_ROOT", "/thumbs")
	t.Setenv("THUMBNAIL_WIDTHS_PX", "")
	t.Setenv("OTEL_ENABLED", "")
	t.Setenv("OTEL_COLLECTOR_GRPC_ENDPOINT", "")

	resetForTests()
	assertPanics(t, func() { AppCfg() })
}

func TestConfigRejectsInvalidThumbnailWidth(t *testing.T) {
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	t.Setenv("DIR_ORIGINALS_ROOT", "/orig")
	t.Setenv("DIR_THUMBNAILS_ROOT", "/thumbs")
	t.Setenv("THUMBNAIL_WIDTHS_PX", "256,abc")

	resetForTests()
	assertPanics(t, func() { AppCfg() })
}

func TestConfigRejectsMissingRequiredRootDirs(t *testing.T) {
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	t.Setenv("DIR_ORIGINALS_ROOT", "")
	t.Setenv("DIR_THUMBNAILS_ROOT", "")
	t.Setenv("THUMBNAIL_WIDTHS_PX", "256")

	resetForTests()
	assertPanics(t, func() { AppCfg() })
}

func TestConfigReturnsSingletonInstance(t *testing.T) {
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	t.Setenv("DIR_ORIGINALS_ROOT", "/orig")
	t.Setenv("DIR_THUMBNAILS_ROOT", "/thumbs")

	resetForTests()
	first := AppCfg()

	t.Setenv("DIR_ORIGINALS_ROOT", "/other")
	second := AppCfg()

	if first != second {
		t.Fatalf("AppCfg() returned different instances: %p vs %p", first, second)
	}
	if got := second.RootDirs.Originals; got != "/orig" {
		t.Fatalf("singleton config changed after init: got %q, want %q", got, "/orig")
	}
}

func resetForTests() {
	instance = nil
	instanceErr = nil
	instanceOnce = sync.Once{}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()

	fn()
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error: %v", dir, err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func writeDotEnv(t *testing.T, dir string, content string) {
	t.Helper()

	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}
