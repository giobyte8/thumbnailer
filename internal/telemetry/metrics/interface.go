package metrics

import (
	"context"
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
)

type MetricsSvc interface {
	Increment(metric MetricName)
	IncrementWAttrs(metric MetricName, attrs map[string]string)
	Shutdown(ctx context.Context) error
}
