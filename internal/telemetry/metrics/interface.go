package metrics

import (
	"context"
)

// Custom type to represent a metric name,
// providing a type-safe way to handle metric names.
type MetricName string

const (
	FileEvtReceived MetricName = "file.event.received"
	ThumbCreated    MetricName = "thumbnail.created"
)

type MetricsSvc interface {
	Increment(metric MetricName, attrs map[string]string)
	Shutdown(ctx context.Context) error
}
