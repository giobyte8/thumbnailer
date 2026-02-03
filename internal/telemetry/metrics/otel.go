package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

type OtelMetricsSvc struct {
	thumbGenRequestReceivedCounter metric.Int64Counter
	thumbDelRequestReceivedCounter metric.Int64Counter
	thumbCreatedCounter            metric.Int64Counter
	shutDownFuncs                  []func(ctx context.Context) error
}

var serviceName = semconv.ServiceNameKey.String("thumbnailer")

func NewOtelMetricsSvc(ctx context.Context) (*OtelMetricsSvc, error) {
	shutDownFuncs, err := initOtel(ctx)
	if err != nil {
		return nil, err
	}
	meter := otel.Meter("thumbnailer")

	thumbGenRequestReceivedCounter, err := meter.Int64Counter(
		string(ThumbGenRequestReceived),
		metric.WithDescription(
			"Number of received 'generate thumbnail' requests'",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	thumbDelRequestReceivedCounter, err := meter.Int64Counter(
		string(ThumbDelRequestReceived),
		metric.WithDescription(
			"Number of received 'delete thumbnail' requests'",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	thumbCreatedCounter, err := meter.Int64Counter(
		string(ThumbCreated),
		metric.WithDescription("Number of created thumbnails"),
		metric.WithUnit("{thumbnail}"),
	)
	if err != nil {
		return nil, err
	}

	return &OtelMetricsSvc{
		thumbGenRequestReceivedCounter: thumbGenRequestReceivedCounter,
		thumbDelRequestReceivedCounter: thumbDelRequestReceivedCounter,
		thumbCreatedCounter:            thumbCreatedCounter,
		shutDownFuncs:                  shutDownFuncs,
	}, nil
}

func (s *OtelMetricsSvc) Increment(metricName MetricName) {
	s.IncrementWAttrs(metricName, nil)
}

func (s *OtelMetricsSvc) IncrementWAttrs(
	metricName MetricName,
	attrs map[string]string,
) {
	if attrs == nil {
		attrs = make(map[string]string)
	}

	// Convert attrs map to OpenTelemetry attributes
	kvAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		kvAttrs = append(kvAttrs, attribute.String(key, value))
	}

	switch metricName {
	case ThumbGenRequestReceived:
		slog.Debug(
			"Incrementing metric",
			"metricName", metricName,
			"attributes", attrs,
		)
		s.thumbGenRequestReceivedCounter.Add(
			context.Background(),
			1,
			metric.WithAttributeSet(attribute.NewSet(kvAttrs...)),
		)
	case ThumbDelRequestReceived:
		s.thumbDelRequestReceivedCounter.Add(
			context.Background(),
			1,
			metric.WithAttributeSet(attribute.NewSet(kvAttrs...)),
		)
	case ThumbCreated:
		s.thumbCreatedCounter.Add(
			context.Background(),
			1,
			metric.WithAttributeSet(attribute.NewSet(kvAttrs...)),
		)
	default:
		slog.Warn("Unknown metric name", "metricName", metricName)
	}
}

func (s *OtelMetricsSvc) Shutdown(ctx context.Context) error {
	for _, shutdownFunc := range s.shutDownFuncs {
		if err := shutdownFunc(ctx); err != nil {
			slog.Error("Error during OpenTelemetry shutdown", "error", err)
			return err
		}
	}

	slog.Debug("OpenTelemetry services shutdown successfully")
	return nil
}

func initOtel(ctx context.Context) ([]func(ctx context.Context) error, error) {
	slog.Debug("Initializing OpenTelemetry")
	var shutDownFuncs []func(ctx context.Context) error

	// Connect to the OpenTelemetry collector
	conn, err := newCollectorGrpcConn()
	if err != nil {
		return nil, err
	}
	//shutDownFuncs = append(shutDownFuncs, conn.Close)

	// Resource for the OpenTelemetry service
	res, err := newResource(ctx)
	if err != nil {
		return nil, err
	}

	meterProvider, err := newMeterProvider(ctx, res, conn)
	if err != nil {
		return nil, err
	}
	shutDownFuncs = append(shutDownFuncs, meterProvider.Shutdown)

	otel.SetMeterProvider(meterProvider)
	return shutDownFuncs, nil
}

func newResource(ctx context.Context) (*resource.Resource, error) {
	res, err := resource.New(ctx, resource.WithAttributes(serviceName))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create resource for OpenTelemetry: %w",
			err,
		)
	}

	return res, nil
}

// Creates a new gRPC connection to the OpenTelemetry collector.
func newCollectorGrpcConn() (*grpc.ClientConn, error) {
	grpc_endpoint := os.Getenv("OTEL_COLLECTOR_GRPC_ENDPOINT")

	conn, err := grpc.NewClient(
		grpc_endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create gRPC connection to collector: %w",
			err,
		)
	}

	return conn, nil
}

func newMeterProvider(
	ctx context.Context,
	res *resource.Resource,
	conn *grpc.ClientConn,
) (*sdkmetric.MeterProvider, error) {
	// metricExporter, err := stdoutmetric.New(
	// 	stdoutmetric.WithPrettyPrint(),
	// )

	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(3*time.Second),
		)),
	)

	return meterProvider, nil
}
