#!/usr/bin/env bash
# Smoke-check a freshly built image.
#
# Validation steps:
# - Prepare env dependencies for the app:
#   - RabbitMQ ephemeral container
# - Start app container
# - Verify app health:
#   - health check endpoint (?)
#   - Verify 'ffmpeg' command is available
#   - Verify 'heif-convert' command is available
#
# Usage:
#   ./image_verify.sh <image_ref>
#
# Optional env vars:
#   SMOKE_START_TIMEOUT_SEC=10
#   SMOKE_POLL_INTERVAL_SEC=2
#   SMOKE_RABBITMQ_IMAGE=rabbitmq:3.13-alpine

set -euo pipefail

IMAGE_REF="${1:-}"
if [ -z "$IMAGE_REF" ]; then
  echo "Usage: ./image_verify.sh <image_ref>"
  exit 1
fi

START_TIMEOUT_SEC="${SMOKE_START_TIMEOUT_SEC:-10}"
POLL_INTERVAL_SEC="${SMOKE_POLL_INTERVAL_SEC:-2}"
RABBITMQ_IMAGE="${SMOKE_RABBITMQ_IMAGE:-rabbitmq:3.13-alpine}"

RUN_ID="${RANDOM}"
NETWORK_NAME="thumb-smoke-net-${RUN_ID}"
APP_CONTAINER="thumb-smoke-app-${RUN_ID}"
RABBITMQ_CONTAINER="thumb-smoke-rabbit-${RUN_ID}"

cleanup() {
  docker rm -f "$APP_CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$RABBITMQ_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "Creating smoke-check network: ${NETWORK_NAME}"
docker network create "$NETWORK_NAME" >/dev/null


echo
echo "Starting RabbitMQ dependency: ${RABBITMQ_CONTAINER} (${RABBITMQ_IMAGE})"
docker run -d \
  --name "$RABBITMQ_CONTAINER" \
  --network "$NETWORK_NAME" \
  --network-alias rabbitmq \
  -e RABBITMQ_DEFAULT_USER=smoke \
  -e RABBITMQ_DEFAULT_PASS=smoke \
  "$RABBITMQ_IMAGE" >/dev/null

# Give it a moment to start up before polling
echo "  Waiting for RabbitMQ readiness"
sleep 4

# Check for readiness by pinging the diagnostics endpoint
start_ts="$(date +%s)"
while true; do
  if docker exec "$RABBITMQ_CONTAINER" \
      rabbitmq-diagnostics -q ping >/dev/null 2>&1; then
    echo "  RabbitMQ is ready ✅"
    break
  fi

  now_ts="$(date +%s)"
  if [ "$((now_ts - start_ts))" -ge "$START_TIMEOUT_SEC" ]; then
    echo "  RabbitMQ did not become ready within ${START_TIMEOUT_SEC}s"
    docker logs "$RABBITMQ_CONTAINER" || true
    exit 1
  fi

  sleep "$POLL_INTERVAL_SEC"
done


echo
echo "Starting app container: ${APP_CONTAINER} from image ${IMAGE_REF}"
docker run -d \
  --name "$APP_CONTAINER" \
  --network "$NETWORK_NAME" \
  -e RABBITMQ_HOST=rabbitmq \
  -e RABBITMQ_PORT=5672 \
  -e RABBITMQ_USER=smoke \
  -e RABBITMQ_PASS=smoke \
  -e RABBITMQ_VHOST="/" \
  -e AMQP_EXCHANGE=GL_EXCHANGE \
  -e AMQP_QUEUE_THUMB_GEN_REQUESTS=GL_GEN_THUMB_REQUESTS \
  -e AMQP_QUEUE_THUMB_DEL_REQUESTS=GL_DEL_THUMB_REQUESTS \
  -e DIR_ORIGINALS_ROOT=/opt/media/originals \
  -e DIR_THUMBNAILS_ROOT=/opt/media/thumbs \
  -e THUMBNAIL_WIDTHS_PX="512,256" \
  -e OTEL_ENABLED=false \
  "${IMAGE_REF}" >/dev/null

# Give it a moment to start up before polling
sleep 2
# TODO Consider adding a health check endpoint to the app and polling it here
# instead of just waiting a fixed time

echo "Verifying ffmpeg availability..."
if docker exec "$APP_CONTAINER" ffmpeg -version >/dev/null 2>&1; then
  echo "  ffmpeg is available ✅"
else
  echo "  ffmpeg is NOT available ❌"
  echo "--- app container logs ---"
  docker logs "$APP_CONTAINER" || true
  exit 1
fi

echo "Verifying heif-convert availability..."
if docker exec "$APP_CONTAINER" heif-convert -version >/dev/null 2>&1; then
  echo "  heif-convert is available ✅"
else
  echo "  heif-convert is NOT available ❌"
  echo "--- app container logs ---"
  docker logs "$APP_CONTAINER" || true
  exit 1
fi

echo
echo "Image verification passed for ${IMAGE_REF}"
