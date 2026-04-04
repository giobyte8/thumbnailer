package metrics

import (
	"context"
	"time"
)

// Custom type to represent a metric name,
// providing a type-safe way to handle metric names.
type MetricName string

const (
	ThumbReqGenReceived MetricName = "thumb.request.generate.received"
	ThumbReqGenRouted   MetricName = "thumb.request.generate.routed"
	ThumbReqDelReceived MetricName = "thumb.request.delete.received"
	ThumbCreated        MetricName = "thumb.created"

	FormatConverted     MetricName = "format_converter.converted"
	VideoFrameExtracted MetricName = "video_frame_extractor.extracted"

	LPDedicatedImageOpsCreated MetricName = "lilliput.dedicated_imageops_created"
	LPErrOutputBufferTooSmall  MetricName = "lilliput.err.output_buffer_too_small"

	// Duration of thumbnail generation including preliminary steps
	// like frame extraction and format conversion.
	ThumbGenerateDuration MetricName = "thumb.generate.duration"

	// Duration of thumbnail generation performed by Lilliput, excluding
	// preliminary steps like frame extraction and format conversion.
	LilptThumbGenDuration MetricName = "lilliput.thumb.generate.duration"

	FormatConvertDuration     MetricName = "format_converter.convert.duration"
	VideoFrameExtractDuration MetricName = "video_frame.extract.duration"
)

type MetricsSvc interface {
	Increment(metric MetricName)
	IncrementWAttrs(metric MetricName, attrs map[string]string)

	Duration(metric MetricName, duration time.Duration)
	DurationWAttrs(
		metric MetricName,
		duration time.Duration,
		attrs map[string]string,
	)

	// DeferredDuration provides a convienient way to measure and record the
	// duration of an operation. It returns a function that, when deferred,
	// will calculate the elapsed time and record it.
	// Usage:
	//   defer metricsSvc.DeferredDuration(metrics.ThumbGenerateDuration)()
	DeferredDuration(metric MetricName) func()

	// DeferredDurationWithAttrs is similar to DeferredDuration but allows
	// for attributes to be included with the duration metric.
	DeferredDurationWAttrs(metric MetricName, attrs map[string]string) func()

	Shutdown(ctx context.Context) error
}
