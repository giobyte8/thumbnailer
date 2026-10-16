# Copilot instructions for thumbnailer project

## Project purpose
files of different formats, using different libraries and tools for it.
It is designed to be containerized and deployed in environments where image
1. The service listens for thumbnail generation requests.
   create thumbnails of specified widths.
3. Generated thumbnails are saved to a designated directory on the filesystem.

- Flow: RabbitMQ message -> `internal/messaging` -> `internal/services`
  (`runtime/thumbs`).
- Data model is intentionally minimal (`internal/models/thumb_requests.go` with
  UUID + relative file path).
  implementation details in generator packages.
  `internal/thumbs_gen/interface.go`, `internal/telemetry/metrics/interface.go`).
## Code Style
- Use structured logging with `log/slog` key/value fields; follow patterns
 AMQP transport + queue binding: `internal/messaging/amqp_resources.go`.
- Prefer constructor + interface boundaries (`NewMessaging`, `NewAMQPConsumer`,
  `ThumbsGenerator`, telemetry interfaces) as in
  `internal/messaging/consumer.go` and `internal/thumbs_gen/interface.go`.
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
  `internal/messaging/queue_consumer.go`.
- Telemetry is backend-switchable (noop vs OTEL) via `OTEL_ENABLED`; see
  `internal/telemetry/telemetry.go`.
- Thumbnail naming follows `<original>_<width>px.<extension>` via
  `internal/thumbs_gen/interface.go`.

## Integration Points
- RabbitMQ transport + queue binding: `internal/messaging/amqp_resources.go`.
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
