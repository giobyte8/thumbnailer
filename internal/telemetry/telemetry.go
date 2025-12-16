package telemetry

import (
	"context"
	"os"

	"github.com/giobyte8/galleries/thumbnailer/internal/telemetry/metrics"
)

type TelemetrySvc struct {
	metrics metrics.MetricsSvc
}

func NewTelemetrySvc(ctx context.Context) (*TelemetrySvc, error) {
	otel_enabled := os.Getenv("OTEL_ENABLED") == "true"
	var metricsSvc metrics.MetricsSvc
	var err error

	if otel_enabled {
		metricsSvc, err = metrics.NewOtelMetricsSvc(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		metricsSvc = metrics.NewNoopMetricsSvc()
	}

	return &TelemetrySvc{
		metrics: metricsSvc,
	}, nil
}

func (t *TelemetrySvc) Metrics() metrics.MetricsSvc {
	return t.metrics
}

func (t *TelemetrySvc) Shutdown(ctx context.Context) error {
	return t.metrics.Shutdown(ctx)
}
