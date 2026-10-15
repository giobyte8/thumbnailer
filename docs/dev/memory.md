# Assessing Memory Consumption

During development we can get insights regarding memory consumption by
running the service, requesting some thumbnails and observing the memory usage
via `ps -o pid=,rss=,vsz=,command= -p <pid>` where `<pid>` is the process ID
of the running service.

Basically:
1. Verify env setup
2. Compile the service
3. Clear the local generation queue
4. Use a script to request thumbnails generation (enqueue messages)
5. Run the service and monitor memory as it consumes/processes requests
6. Monitor the queue until all requests are processed

## Verify env setup
Verify that `.env` file points to the correct paths.
You can comment existing config and replace it with test config:
```env

# Use the `troubleshoot` directory for testing purposes
DIR_ORIGINALS_ROOT=/Users/giobyte8/src/thumbnailer/runtime/originals/troubleshoot

# Test thumbnails can be stored to `runtime/thumbs` directory
DIR_THUMBNAILS_ROOT=/Users/giobyte8/src/thumbnailer/runtime/thumbs
```

## Compile the service
Run the following command to compile the service:
```bash
go build -o ./build/thumbnailer ./cmd/thumbnailer
```

## Clear the generation queue
Before adding requests, clear messages left by a previous test run:
```bash
source .env

curl -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
  -X DELETE \
  "http://$RABBITMQ_HOST:${RABBITMQ_API_PORT:-15672}/api/queues/%2F/$AMQP_QUEUE_THUMB_GEN_REQUESTS/contents"
```

This deletes all messages in the configured generation queue. Only run it
against the local development queue.

## Request thumbnails generation
Use the provided script to enqueue one message for every image in the
`troubleshoot` directory:
```bash
./scripts/req_thumbs_generate.sh
```

- It reads `.env` to determine paths and destination queue.
- Since app is not running, messages get enqueued.

## Monitor queue completion
In a separate terminal, start this monitor before running the service. It shows
the number of ready and unacknowledged generation requests:
```bash
source .env

while true; do
  clear
  curl -s -u "$RABBITMQ_USER:$RABBITMQ_PASS" \
    "http://$RABBITMQ_HOST:${RABBITMQ_API_PORT:-15672}/api/queues/%2F/$AMQP_QUEUE_THUMB_GEN_REQUESTS" \
    | jq '{messages, messages_ready, messages_unacknowledged}'
  sleep 0.2
done
```

The batch is complete when `messages`, `messages_ready`, and
`messages_unacknowledged` are all `0`. Press `Ctrl+C` to stop the monitor.

## Run the service and monitor memory

```shell
./build/thumbnailer &
pid=$!
echo "Thumbnailer PID: $pid"

while kill -0 "$pid" 2>/dev/null; do
  date '+%H:%M:%S'
  ps -o pid=,rss=,vsz=,command= -p "$pid"
  sleep 0.2
done
```

For the automated local assessment, run:

```shell
./.github/skills/assess-memory-consumption/scripts/measure.sh
```

It waits briefly for RabbitMQ to register the batch, measures while the queue
drains, and limits service shutdown to five seconds. Pass a positive timeout
in seconds only when the workload needs longer than the default 60 seconds.
