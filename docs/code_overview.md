# Code Overview

## Index

- [Architecture and Code Organization](#architecture-and-code-organization)
- [Abstractions and Extensibility](#abstractions-and-extensibility)

## Architecture and Code Organization

The Thumbnailer service processes 'thumbnail requests' and generates thumbnails
for requested files. The codebase is organized into the following components:

- **Main Entry Point**
  - Located in `cmd/thumbnailer/main.go`
  - Initializes dependencies and starts the service

- **Consumer**
  - `internal/consumer`
  - Listens to RabbitMQ for requests
  - Processes messages using `AMQPConsumer`

- **Services**
  - `internal/services`
  - Orchestrates thumbnail generation
  - Handles cleanup and metadata preparation

- **Thumbnail Generation**
  - `internal/thumbs_gen`
  - Defines `ThumbsGenerator` interface
  - Current implementation: `LilliputThumbsGenerator` (uses Lilliput library)

- **Models**
  - `internal/models`
  - Internal Data structures

- **Scripts**
  - `scripts`
  - Utilities for development and testing (e.g., `fetch_images.sh`, `gen_amqp_traffic.sh`)

- **Configuration**
  - Managed via environment variables
  - `.env` file support with `template.env` as a reference

## Abstractions and Extensibility

| Interface          | Current Implementation       | Purpose                                      |
|--------------------|------------------------------|----------------------------------------------|
| `MessageConsumer`  | `AMQPConsumer`               | Consumes messages from RabbitMQ              |
| `ThumbsGenerator`  | `LilliputThumbsGenerator`    | Generates thumbnails using Lilliput library  |