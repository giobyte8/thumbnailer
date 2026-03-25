# Copilot instructions for thumbnailer project

## Project purpose
This project implements a thumbnail generation service, for image and video
files of different formats, using different libraries and tools for it.

It is designed to be containerized and deployed in environments where image
processing is needed, such as web applications or media management systems.

Main runtime flow:
1. The service listens for thumbnail generation requests.
2. Upon receiving a request, it processes the original image/video file to
   create thumbnails of specified widths.
3. Generated thumbnails are saved to a designated directory on the filesystem.

## Architecture
- Entrypoint: `cmd/thumbnailer/main.go` wires env, logging, telemetry,
  consumer, and graceful shutdown.
- Flow: RabbitMQ message -> `internal/consumer` -> `internal/services`
  orchestration -> `internal/thumbs_gen` image generation -> filesystem output
  (`runtime/thumbs`).
- Data model is intentionally minimal (`internal/models/thumb_requests.go` with
  UUID + relative file path).
- Keep transport concerns in consumer, business orchestration in services,
  implementation details in generator packages.
- Prefer interfaces over concrete coupling (`internal/consumer/consumer.go`,
  `internal/thumbs_gen/interface.go`, `internal/telemetry/metrics/interface.go`).

## Code Style
- Language: Go (module in `go.mod`, toolchain pinned in `mise.toml`).
- Use structured logging with `log/slog` key/value fields; follow patterns
  in `cmd/thumbnailer/main.go` and `internal/consumer/amqp_consumer.go`.
- Prefer constructor + interface boundaries (`NewAMQPConsumer`,
  `ThumbsGenerator`, telemetry interfaces) as in
  `internal/consumer/consumer.go` and `internal/thumbs_gen/interface.go`.
- Keep error handling explicit and wrapped (`fmt.Errorf("...: %w", err)`),
  matching `internal/services/thumbnails.go` and
  `internal/thumbs_gen/lilliput_generator.go`.

## Build and Test
- Setup: `cp template.env .env && vim .env`
- Dependencies: `go mod tidy`
- Run service: `go run ./cmd/thumbnailer`

## Project Conventions
- Config is env-first; `.env` loading is optional but missing required dirs is
  fatal (`DIR_ORIGINALS_ROOT`, `DIR_THUMBNAILS_ROOT`).
- Queue names are explicit env vars and bound with routing key == queue name
  (`AMQP_QUEUE_THUMB_GEN_REQUESTS`, `AMQP_QUEUE_THUMB_DEL_REQUESTS`).
- Message processing uses manual AMQP ack/nack (auto-ack disabled) in
  `internal/consumer/amqp_consumer.go`.
- Telemetry is backend-switchable (noop vs OTEL) via `OTEL_ENABLED`; see
  `internal/telemetry/telemetry.go`.
- Thumbnail naming follows `<original>_<width>px.<extension>` via
  `internal/thumbs_gen/interface.go`.

## Integration Points
- RabbitMQ transport + queue binding: `internal/consumer/amqp_consumer.go`.
- Image generation backend: `internal/thumbs_gen/lilliput_generator.go`.
- Telemetry backends: `internal/telemetry/metrics/noop.go` and
  `internal/telemetry/metrics/otel.go`.
- Dev helper scripts: `scripts/req_thumbs_generate.sh`,
  `scripts/req_thumbs_delete.sh`, `scripts/fetch_images.sh`.

## Security
- Treat AMQP payload fields as untrusted input; preserve strict unmarshal +
  nack behavior in consumer.
- Be careful with `req.FilePath` joins in `internal/services/thumbnails.go`
  / generator paths; avoid introducing path traversal risk.
- Do not hardcode credentials; keep secrets in env (`RABBITMQ_USER`,
  `RABBITMQ_PASS`, OTEL endpoint).
- Avoid logging sensitive payload content beyond what is necessary for
  debugging.
