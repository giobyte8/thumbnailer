#!/bin/bash
# Generates AMQP messages for development and testing purposes.
# It generates messages for the following queues:
#   - ${AMQP_QUEUE_DISCOVERED_FILES} - One message per file at /runtime/originals

function json_escape() {
  printf '%s' "$1" | python -c 'import json,sys; print(json.dumps(sys.stdin.read()))'
}

SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
CALLER_DIR="$(pwd)"
cd "$SCRIPT_DIR"

# Load .env file if it exists
if [ -f "../.env" ]; then
    source ../.env
fi

ORIGINALS=$(realpath "$DIR_ORIGINALS_ROOT")
if [ ! -d "$ORIGINALS" ]; then
    echo "Directory $ORIGINALS does not exist. Please run fetch_images.sh first."
    exit 1
fi

RABBITMQ_API_PORT=${RABBITMQ_API_PORT:-15672}
scan_req_uuid=$(uuidgen)

# Iterate files in the originals directory
for file in "$ORIGINALS"/*; do
    filename=$(basename "$file")

    # Prepare message payload
    msg="{
        \"scanRequestId\": \"$scan_req_uuid\",
        \"eventType\": \"NEW_FILE_FOUND\",
        \"filePath\": \"$filename\"
    }"
    j_msg=$(json_escape "$msg")

    amqp_msg="{
        \"properties\": {},
        \"routing_key\": \"$AMQP_QUEUE_DISCOVERED_FILES\",
        \"payload\": $j_msg,
        \"payload_encoding\": \"string\"
    }"

    # Post message to RabbitMQ
    echo "Posting AMQP message for: $filename"
    curl -s \
        -u "$RABBITMQ_USER:$RABBITMQ_PASS"  \
        -X POST                                     \
        -d "$amqp_msg"                              \
        http://$RABBITMQ_HOST:$RABBITMQ_API_PORT/api/exchanges/%2F/$AMQP_EXCHANGE_GALLERIES/publish

    # Add missing line break for readability
    echo

    # If got param '--single', break after the first file
    if [[ "$1" == "--single" ]]; then
        echo "Single mode enabled, stopping after the first file."
        break
    fi
done

cd "$CALLER_PATH"
