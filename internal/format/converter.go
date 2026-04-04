package format

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

type FormatConverter struct {
	telemetry      *telemetry.TelemetrySvc
	formatDetector *FormatDetector
}

func NewFormatConverter(
	telemetrySvc *telemetry.TelemetrySvc,
	formatDetector *FormatDetector,
) *FormatConverter {
	return &FormatConverter{
		telemetry:      telemetrySvc,
		formatDetector: formatDetector,
	}
}

// Convert converts the file at 'srcAbsPath' to the specified 'dstFormat'
// and saves it at 'dstAbsPath'.
//
// Note: this method checks if the formats of source and destination files are
// supported before performing the conversion. Use ConvertWithoutFormatsCheck
// if you want to skip such checks.
func (c *FormatConverter) Convert(
	ctx context.Context,
	srcAbsPath string,
	dstAbsPath string,
	dstFormat Format,
) error {
	err := c.isConversionSupported(srcAbsPath, dstAbsPath, dstFormat)
	if err != nil {
		return fmt.Errorf("conversion not supported: %w", err)
	}

	return c.ConvertWithoutFormatsCheck(ctx, srcAbsPath, dstAbsPath, dstFormat)
}

// ConvertWithoutFormatsCheck performs the format conversion without
// checking for supported formats in origin and destination files.
//
// Use this for high throughput scenarios where formats were
// already checked by the caller
func (c *FormatConverter) ConvertWithoutFormatsCheck(
	ctx context.Context,
	srcAbsPath string,
	dstAbsPath string,
	dstFormat Format,
) error {
	startTime := time.Now()

	// Prepare 'heif-convert' command to do the conversion
	//   Use 'heif-convert --help' for usage information
	args := []string{"-q 75", srcAbsPath, dstAbsPath}
	command := exec.CommandContext(ctx, "heif-convert", args...)

	output, err := command.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("heif-convert binary not found: %w", err)
		}

		return fmt.Errorf(
			"heif-convert failed for %s: %w. output: %s",
			srcAbsPath,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	c.telemetry.Metrics().Duration(
		metrics.FormatConvertDuration,
		time.Since(startTime))
	c.telemetry.Metrics().Increment(metrics.FormatConverted)
	return nil
}

func (c *FormatConverter) isConversionSupported(
	srcAbsPath string,
	dstAbsPath string,
	dstFormat Format,
) error {

	// Validate dst extension is supported
	if !c.isDstExtensionSupported(dstAbsPath) {
		return fmt.Errorf(
			"unsupported destination file extension: %s",
			filepath.Ext(dstAbsPath))
	}

	// Validate dst format is supported
	if !c.isDstFormatSupported(dstFormat) {
		return fmt.Errorf("unsupported destination format: %v", dstFormat)
	}

	// Detect src format
	format, err := c.formatDetector.Detect(srcAbsPath)
	if err != nil {
		return fmt.Errorf("failed to detect format of source file: %w", err)
	}

	// Validate src format is supported
	if !c.isSrcFormatSupported(format) {
		return fmt.Errorf("unsupported source format: %v", format)
	}

	return nil
}

func (c *FormatConverter) isSrcFormatSupported(srcFormat Format) bool {

	// Only HEIF is supported for now
	return HEIF == srcFormat
}

func (c *FormatConverter) isDstFormatSupported(dstFormat Format) bool {

	// Only JPEG is supported for now
	return JPEG == dstFormat
}

func (c *FormatConverter) isDstExtensionSupported(dstAbsPath string) bool {

	// 'heif-convert' supported extensions
	//   Use 'heif-convert --help' to see supported output file extensions
	supportedExtensions := []string{".jpg", ".jpeg"}

	dstExt := strings.ToLower(filepath.Ext(dstAbsPath))
	return slices.Contains(supportedExtensions, dstExt)
}
