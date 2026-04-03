package frameextractor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

type Extractor struct {
	telemetry      *telemetry.TelemetrySvc
	formatDetector *format.FormatDetector
}

func NewFrameExtractor(
	telemetrySvc *telemetry.TelemetrySvc,
	formatDetector *format.FormatDetector,
) *Extractor {
	return &Extractor{
		telemetry:      telemetrySvc,
		formatDetector: formatDetector,
	}
}

// Extract extracts a frame from the video at 'fromAbsPath' and saves it as an
// image at 'intoAbsPath'.
//
// Note: this method checks if the formats of source and destination files are
// supported before performing the extraction. Use ExtractWithoutFormatsCheck
// if you want to skip such checks.
func (e *Extractor) Extract(
	ctx context.Context,
	fromAbsPath string,
	intoAbsPath string,
) error {
	err := e.isExtractionSupported(fromAbsPath, intoAbsPath)
	if err != nil {
		return fmt.Errorf("frame extraction not supported: %w", err)
	}

	return e.ExtractWithoutFormatsCheck(ctx, fromAbsPath, intoAbsPath)
}

// ExtractWithoutFormatsCheck performs the frame extraction without checking
// for supported formats in origin and destination files.
//
// Use this for high throughput scenarios where formats were
// already checked by the caller
func (e *Extractor) ExtractWithoutFormatsCheck(
	ctx context.Context,
	fromAbsPath string,
	intoAbsPath string,
) error {

	cmd := e.makeFFmpegCommand(ctx, fromAbsPath, intoAbsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("ffmpeg binary not found: %w", err)
		}

		return fmt.Errorf(
			"ffmpeg frame extraction failed for %s: %w. output: %s",
			fromAbsPath,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	e.telemetry.Metrics().Increment(metrics.VideoFrameExtracted)
	return nil
}

// Prepare 'ffmpeg' command to do the frame extraction.
// Use 'ffmpeg -h' for usage information
func (e *Extractor) makeFFmpegCommand(
	ctx context.Context,
	fromAbsPath string,
	intoAbsPath string,
) *exec.Cmd {
	args := []string{
		"-y",
		"-ss", "00:00:01",
		"-i", fromAbsPath,
		"-vframes", "1",
		"-vf", "format=yuv420p",
		"-q:v", "2",
		intoAbsPath,
	}

	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func (e *Extractor) isExtractionSupported(
	fromAbsPath string,
	intoAbsPath string,
) error {

	// Validate dst extension is supported
	if !e.isDstExtensionSupported(intoAbsPath) {
		return fmt.Errorf(
			"unsupported destination file extension: %s",
			filepath.Ext(intoAbsPath))
	}

	// Detect src format
	format, err := e.formatDetector.Detect(fromAbsPath)
	if err != nil {
		return fmt.Errorf("failed to detect format of source file: %w", err)
	}

	// Validate src format is supported
	if !e.isSrcFormatSupported(format) {
		return fmt.Errorf("unsupported source format: %v", format)
	}

	return nil
}

func (e *Extractor) isSrcFormatSupported(fromFormat format.Format) bool {

	// Only MOV, MP4 and M4V are supported for now
	supportedFormats := []format.Format{format.MOV, format.MP4, format.M4V}
	return slices.Contains(supportedFormats, fromFormat)
}

func (e *Extractor) isDstExtensionSupported(intoAbsPath string) bool {

	// Only JPEG is supported for now.
	//   Technically ffmpeg supports more formats, but we only want
	//   to support JPEG to keep the scope of this extractor limited.
	supportedExtensions := []string{".jpg", ".jpeg"}

	dstExt := strings.ToLower(filepath.Ext(intoAbsPath))
	return slices.Contains(supportedExtensions, dstExt)
}
