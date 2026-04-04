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
	counters   otelCounters
	histograms otelHistograms

	shutDownFuncs []func(ctx context.Context) error
}

type otelCounters struct {
	thumbReqGenReceivedCounter metric.Int64Counter
	thumbReqGenRoutedCounter   metric.Int64Counter
	thumbReqDelReceivedCounter metric.Int64Counter
	thumbCreatedCounter        metric.Int64Counter

	formatConvertedCounter     metric.Int64Counter
	videoFrameExtractedCounter metric.Int64Counter

	lpDedicatedImageOpsCreatedCounter metric.Int64Counter
	lpErrOutputBufferTooSmallCounter  metric.Int64Counter
}

type otelHistograms struct {
	thumbGenerateDurationHistogram     metric.Int64Histogram
	lilptThumbGenDurationHistogram     metric.Int64Histogram
	videoFrameExtractDurationHistogram metric.Int64Histogram
	formatConvertDurationHistogram     metric.Int64Histogram
}

var serviceName = semconv.ServiceNameKey.String("thumbnailer")

func NewOtelMetricsSvc(ctx context.Context) (*OtelMetricsSvc, error) {
	shutDownFuncs, err := initOtel(ctx)
	if err != nil {
		return nil, err
	}
	meter := otel.Meter("thumbnailer")

	counters, err := initCounters(meter)
	if err != nil {
		return nil, err
	}

	histograms, err := initHistograms(meter)
	if err != nil {
		return nil, err
	}

	return &OtelMetricsSvc{
		counters:      *counters,
		histograms:    *histograms,
		shutDownFuncs: shutDownFuncs,
	}, nil
}

func initCounters(meter metric.Meter) (*otelCounters, error) {
	thumbReqGenReceivedCounter, err := meter.Int64Counter(
		string(ThumbReqGenReceived),
		metric.WithDescription(
			"Number of received 'generate thumbnail' requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	thumbReqGenRoutedCounter, err := meter.Int64Counter(
		string(ThumbReqGenRouted),
		metric.WithDescription(
			"Number of 'generate thumbnail' requests routed by format"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	thumbReqDelReceivedCounter, err := meter.Int64Counter(
		string(ThumbReqDelReceived),
		metric.WithDescription(
			"Number of received 'delete thumbnail' requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	thumbCreatedCounter, err := meter.Int64Counter(
		string(ThumbCreated),
		metric.WithDescription(
			"Number of created thumbnails"),
		metric.WithUnit("{thumbnail}"),
	)
	if err != nil {
		return nil, err
	}

	formatConvertedCounter, err := meter.Int64Counter(
		string(FormatConverted),
		metric.WithDescription(
			"Number of files converted from one format to another"),
		metric.WithUnit("{file}"),
	)
	if err != nil {
		return nil, err
	}

	videoFrameExtractedCounter, err := meter.Int64Counter(
		string(VideoFrameExtracted),
		metric.WithDescription(
			"Number of times a frame was extracted from a video"),
		metric.WithUnit("{video}"),
	)
	if err != nil {
		return nil, err
	}

	lpDedicatedImageOpsCreatedCounter, err := meter.Int64Counter(
		string(LPDedicatedImageOpsCreated),
		metric.WithDescription(
			"Number of dedicated image operations created in Lilliput"),
		metric.WithUnit("{imageOps}"),
	)
	if err != nil {
		return nil, err
	}

	lpErrOutputBufferTooSmallCounter, err := meter.Int64Counter(
		string(LPErrOutputBufferTooSmall),
		metric.WithDescription(
			"Number of times Lilliput returned 'output buffer too small'"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &otelCounters{
		thumbReqGenReceivedCounter: thumbReqGenReceivedCounter,
		thumbReqGenRoutedCounter:   thumbReqGenRoutedCounter,
		thumbReqDelReceivedCounter: thumbReqDelReceivedCounter,
		thumbCreatedCounter:        thumbCreatedCounter,

		formatConvertedCounter:     formatConvertedCounter,
		videoFrameExtractedCounter: videoFrameExtractedCounter,

		lpDedicatedImageOpsCreatedCounter: lpDedicatedImageOpsCreatedCounter,
		lpErrOutputBufferTooSmallCounter:  lpErrOutputBufferTooSmallCounter,
	}, nil
}

func initHistograms(meter metric.Meter) (*otelHistograms, error) {
	thumbGenerateDurationHistogram, err := meter.Int64Histogram(
		string(ThumbGenerateDuration),
		metric.WithDescription(
			"Duration of thumbnail generation requests"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	lilptThumbGenDurationHistogram, err := meter.Int64Histogram(
		string(LilptThumbGenDuration),
		metric.WithDescription(
			"Duration of thumbnail generation performed by Lilliput"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	videoFrameExtractDurationHistogram, err := meter.Int64Histogram(
		string(VideoFrameExtractDuration),
		metric.WithDescription(
			"Duration of video frame extraction operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	formatConvertDurationHistogram, err := meter.Int64Histogram(
		string(FormatConvertDuration),
		metric.WithDescription(
			"Duration of format conversion operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	return &otelHistograms{
		thumbGenerateDurationHistogram:     thumbGenerateDurationHistogram,
		lilptThumbGenDurationHistogram:     lilptThumbGenDurationHistogram,
		videoFrameExtractDurationHistogram: videoFrameExtractDurationHistogram,
		formatConvertDurationHistogram:     formatConvertDurationHistogram,
	}, nil
}

func (s *OtelMetricsSvc) Increment(metricName MetricName) {
	s.IncrementWAttrs(metricName, nil)
}

func (s *OtelMetricsSvc) IncrementWAttrs(
	metricName MetricName,
	attrs map[string]string,
) {
	ctx := context.Background()
	opts := metric.WithAttributeSet(toAttributeSet(attrs))

	switch metricName {
	case ThumbReqGenReceived:
		s.counters.thumbReqGenReceivedCounter.Add(ctx, 1, opts)
	case ThumbReqGenRouted:
		s.counters.thumbReqGenRoutedCounter.Add(ctx, 1, opts)
	case ThumbReqDelReceived:
		s.counters.thumbReqDelReceivedCounter.Add(ctx, 1, opts)
	case ThumbCreated:
		s.counters.thumbCreatedCounter.Add(ctx, 1, opts)

	case FormatConverted:
		s.counters.formatConvertedCounter.Add(ctx, 1, opts)
	case VideoFrameExtracted:
		s.counters.videoFrameExtractedCounter.Add(ctx, 1, opts)

	case LPDedicatedImageOpsCreated:
		s.counters.lpDedicatedImageOpsCreatedCounter.Add(ctx, 1, opts)
	case LPErrOutputBufferTooSmall:
		s.counters.lpErrOutputBufferTooSmallCounter.Add(ctx, 1, opts)

	default:
		slog.Warn("Unknown metric name", "metricName", metricName)
	}
}

func (s *OtelMetricsSvc) Duration(
	metricName MetricName,
	duration time.Duration,
) {
	s.DurationWAttrs(metricName, duration, nil)
}

func (s *OtelMetricsSvc) DurationWAttrs(
	metricName MetricName,
	duration time.Duration,
	attrs map[string]string,
) {
	durationMs := duration.Milliseconds()

	ctx := context.Background()
	opts := metric.WithAttributeSet(toAttributeSet(attrs))

	switch metricName {
	case ThumbGenerateDuration:
		s.histograms.thumbGenerateDurationHistogram.Record(
			ctx,
			durationMs,
			opts,
		)
	case LilptThumbGenDuration:
		s.histograms.lilptThumbGenDurationHistogram.Record(
			ctx,
			durationMs,
			opts,
		)
	case VideoFrameExtractDuration:
		s.histograms.videoFrameExtractDurationHistogram.Record(
			ctx,
			durationMs,
			opts,
		)
	case FormatConvertDuration:
		s.histograms.formatConvertDurationHistogram.Record(
			ctx,
			durationMs,
			opts,
		)

	default:
		slog.Warn("Unknown duration metric name", "metricName", metricName)
	}
}

// DeferredDuration implements MetricsSvc.
func (s *OtelMetricsSvc) DeferredDuration(metric MetricName) func() {
	return s.DeferredDurationWAttrs(metric, nil)
}

// DeferredDurationWAttrs implements MetricsSvc.
func (s *OtelMetricsSvc) DeferredDurationWAttrs(
	metric MetricName,
	attrs map[string]string,
) func() {
	start := time.Now()

	// Code inside 'func()' will be executed until the caller defers it,
	// allowing us to measure the duration of the operation.
	return func() {
		duration := time.Since(start)
		s.DurationWAttrs(metric, duration, attrs)
	}
}

func toAttributeSet(attrs map[string]string) attribute.Set {
	if attrs == nil {
		attrs = make(map[string]string)
	}

	kvAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		kvAttrs = append(kvAttrs, attribute.String(key, value))
	}

	return attribute.NewSet(kvAttrs...)
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
