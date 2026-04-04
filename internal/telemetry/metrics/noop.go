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

func (n *NoopMetricsSvc) DurationWAttrs(
	metric MetricName,
	duration time.Duration,
	attrs map[string]string) {
	// No operation performed
}

func (n *NoopMetricsSvc) DeferredDuration(metric MetricName) func() {
	// Return a no-op function since we're not tracking duration
	return func() {}
}

func (n *NoopMetricsSvc) DeferredDurationWAttrs(
	metric MetricName,
	attrs map[string]string,
) func() {
	// Return a no-op function since we're not tracking duration
	return func() {}
}

func (n *NoopMetricsSvc) Shutdown(ctx context.Context) error {
	return nil
}
