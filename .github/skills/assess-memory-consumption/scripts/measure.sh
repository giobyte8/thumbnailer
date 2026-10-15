#!/usr/bin/env bash
set -euo pipefail

readonly timeout_seconds="${1:-60}"
readonly sample_interval_seconds="0.2"
readonly shutdown_timeout_seconds="5"
readonly enqueue_timeout_seconds="5"
readonly queue_vhost="%2F"

if ! [[ "$timeout_seconds" =~ ^[1-9][0-9]*$ ]]; then
  echo "Timeout must be a positive whole number of seconds." >&2
  exit 1
fi

for command in curl go jq ps; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Required command not found: $command" >&2
    exit 1
  fi
done

if [[ ! -f .env ]]; then
  echo "Missing .env file." >&2
  exit 1
fi

# shellcheck disable=SC1091
source .env

: "${RABBITMQ_HOST:?RABBITMQ_HOST must be set in .env}"
: "${RABBITMQ_USER:?RABBITMQ_USER must be set in .env}"
: "${RABBITMQ_PASS:?RABBITMQ_PASS must be set in .env}"
: "${AMQP_QUEUE_THUMB_GEN_REQUESTS:?AMQP_QUEUE_THUMB_GEN_REQUESTS must be set in .env}"
: "${DIR_ORIGINALS_ROOT:?DIR_ORIGINALS_ROOT must be set in .env}"

readonly rabbitmq_api_port="${RABBITMQ_API_PORT:-15672}"
readonly queue_url="http://${RABBITMQ_HOST}:${rabbitmq_api_port}/api/queues/${queue_vhost}/${AMQP_QUEUE_THUMB_GEN_REQUESTS}"
readonly originals_dir="$DIR_ORIGINALS_ROOT"

if [[ ! -d "$originals_dir" ]]; then
  echo "Originals directory does not exist: $originals_dir" >&2
  exit 1
fi

shopt -s nullglob
images=("$originals_dir"/*)
shopt -u nullglob
if (( ${#images[@]} == 0 )); then
  echo "No test images found in: $originals_dir" >&2
  exit 1
fi

service_pid=""
cleanup_complete=false
cleanup() {
  if [[ "$cleanup_complete" == true ]]; then
    return
  fi
  cleanup_complete=true

  if [[ -z "$service_pid" ]] || ! kill -0 "$service_pid" 2>/dev/null; then
    return
  fi

  kill -TERM "$service_pid" 2>/dev/null || true
  local shutdown_deadline=$((SECONDS + shutdown_timeout_seconds))
  while kill -0 "$service_pid" 2>/dev/null && (( SECONDS < shutdown_deadline )); do
    sleep "$sample_interval_seconds"
  done

  if kill -0 "$service_pid" 2>/dev/null; then
    kill -KILL "$service_pid" 2>/dev/null || true
  fi

  wait "$service_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

queue_state() {
  curl --fail --silent --show-error \
    -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
    "$queue_url" \
    | jq -r '[.messages, .messages_ready, .messages_unacknowledged] | @tsv'
}

queue_is_empty() {
  local state
  state="$(queue_state)"
  [[ "$state" == $'0\t0\t0' ]]
}

queue_has_messages() {
  local state
  state="$(queue_state)"
  [[ "$state" != $'0\t0\t0' ]]
}

echo "Building thumbnailer..."
go build -o ./build/thumbnailer ./cmd/thumbnailer

echo "Clearing generation queue..."
curl --fail --silent --show-error \
  -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
  -X DELETE \
  "${queue_url}/contents" >/dev/null

echo "Enqueueing ${#images[@]} test image(s)..."
./scripts/req_thumbs_generate.sh >/dev/null

enqueue_deadline=$((SECONDS + enqueue_timeout_seconds))
while ! queue_has_messages && (( SECONDS < enqueue_deadline )); do
  sleep "$sample_interval_seconds"
done

if ! queue_has_messages; then
  echo "No generation requests were enqueued." >&2
  exit 1
fi

echo "Starting thumbnailer..."
./build/thumbnailer >/tmp/thumbnailer-memory-assessment.log 2>&1 &
service_pid=$!

minimum_rss_kib=""
maximum_rss_kib=0
deadline=$((SECONDS + timeout_seconds))
queue_drained=false

# Allow the process to finish exec before collecting RSS samples.
sleep "$sample_interval_seconds"

while (( SECONDS < deadline )); do
  if ! kill -0 "$service_pid" 2>/dev/null; then
    wait "$service_pid" || true
    echo "Thumbnailer exited before the queue drained. See /tmp/thumbnailer-memory-assessment.log" >&2
    exit 1
  fi

  rss_kib="$(ps -o rss= -p "$service_pid" | tr -d '[:space:]')"
  if [[ "$rss_kib" =~ ^[0-9]+$ ]]; then
    if [[ -z "$minimum_rss_kib" ]] || (( rss_kib < minimum_rss_kib )); then
      minimum_rss_kib=$rss_kib
    fi
    if (( rss_kib > maximum_rss_kib )); then
      maximum_rss_kib=$rss_kib
    fi
  fi

  if queue_is_empty; then
    queue_drained=true
    break
  fi

  sleep "$sample_interval_seconds"
done

if [[ "$queue_drained" != true ]]; then
  echo "Generation queue did not drain within ${timeout_seconds} seconds. See /tmp/thumbnailer-memory-assessment.log" >&2
  exit 1
fi

if [[ -z "$minimum_rss_kib" ]]; then
  echo "No RSS samples were collected." >&2
  exit 1
fi

awk -v min_kib="$minimum_rss_kib" -v max_kib="$maximum_rss_kib" 'BEGIN {
  printf "minimum_rss_kib=%d\n", min_kib
  printf "minimum_rss_mib=%.2f\n", min_kib / 1024
  printf "maximum_rss_kib=%d\n", max_kib
  printf "maximum_rss_mib=%.2f\n", max_kib / 1024
}'
