package metrics

import (
	"context"
)

type NoopMetricsSvc struct{}

func NewNoopMetricsSvc() *NoopMetricsSvc {
	return &NoopMetricsSvc{}
}

func (n *NoopMetricsSvc) Increment(
	metric MetricName,
	attrs map[string]string) {
	// No operation performed
}

func (n *NoopMetricsSvc) Shutdown(ctx context.Context) error {
	return nil
}
