package metrics

import (
	"context"
	"time"
)

type NoopMetricsSvc struct{}

func NewNoopMetricsSvc() *NoopMetricsSvc {
	return &NoopMetricsSvc{}
}

func (n *NoopMetricsSvc) Increment(metric MetricName) {
	// No operation performed
}

func (n *NoopMetricsSvc) IncrementWAttrs(
	metric MetricName,
	attrs map[string]string) {
	// No operation performed
}

func (n *NoopMetricsSvc) Duration(metric MetricName, duration time.Duration) {
	// No operation performed
}

func (n *NoopMetricsSvc) Shutdown(ctx context.Context) error {
	return nil
}
