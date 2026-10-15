---
name: assess-memory-consumption
description: "Measure local thumbnailer memory consumption while generating troubleshooting thumbnails. Use when asked for a memory assessment, memory consumption test, RSS measurement, or memory verification for thumbnail generation. Builds the service, processes all test images in runtime/originals/troubleshoot, and reports minimum and maximum RSS only."
argument-hint: "Optional timeout in seconds (default: 60)"
---

# Assess Memory Consumption

Measure the local thumbnailer service while it processes all images in
`runtime/originals/troubleshoot`.

## Run

```bash
./.github/skills/assess-memory-consumption/scripts/measure.sh
```

The optional argument is the processing timeout in seconds; it defaults to
`60`. The script confirms that RabbitMQ has registered the test batch before
starting the service, then stops that service with a five-second shutdown
limit once the queue drains.

The script clears the configured local generation queue before enqueueing the
test batch. Report only minimum and maximum RSS in KiB and MiB. On failure,
report the error without inferring a measurement.

For the manual procedure or troubleshooting, see [memory.md](../../../docs/dev/memory.md).
