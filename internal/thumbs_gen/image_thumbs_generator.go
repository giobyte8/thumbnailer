package thumbsgen

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/discord/lilliput"
	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

type ImageThumbsGenerator struct {
	telemetry                *telemetry.TelemetrySvc
	formatConverter          *format.FormatConverter
	formatDetector           *format.FormatDetector
	maxAllowedImageDimension int
	lpResourceManager        *LPResourceManager
}

// NewImageThumbsGenerator builds an image thumbnail generator with
// explicit dependencies.
func NewImageThumbsGenerator(
	telemetry *telemetry.TelemetrySvc,
	formatConverter *format.FormatConverter,
	formatDetector *format.FormatDetector,
	maxAllowedImageDimension int,
) (*ImageThumbsGenerator, error) {
	lpResourceManager, err := NewLPResourceManager(telemetry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Lilliput resource manager: %w", err)
	}

	return &ImageThumbsGenerator{
		telemetry:                telemetry,
		formatConverter:          formatConverter,
		formatDetector:           formatDetector,
		maxAllowedImageDimension: maxAllowedImageDimension,
		lpResourceManager:        lpResourceManager,
	}, nil
}

// Generate implements ThumbsGenerator.
func (g *ImageThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	origFileAbsPath := mkOriginalFileAbsPath(meta)

	// Detect original file format
	origFileFormat, err := g.formatDetector.Detect(origFileAbsPath)
	if err != nil {
		return fmt.Errorf(
			"failed to detect file format for %s: %w",
			meta.OrigFileRelPath,
			err)
	}

	// Validate original file format is supported
	err = g.isOriginalFileFormatSupported(origFileFormat)
	if err != nil {
		return fmt.Errorf("cannot generate thumbnails: %w", err)
	}

	return g.GenerateWithoutFormatsCheck(ctx, meta, origFileFormat)
}

// GenerateWithoutFormatsCheck implements ThumbsGenerator.
func (g *ImageThumbsGenerator) GenerateWithoutFormatsCheck(
	ctx context.Context,
	meta ThumbnailMeta,
	origFileFormat format.Format,
) error {

	// If original file is HEIF, convert it to JPEG first and use the
	// converted file as input for thumbnail generation.
	if format.HEIF == origFileFormat {
		intermediaryFileAbsPath, err := g.mkIntermediaryFile(ctx, meta)
		if err != nil {
			return fmt.Errorf(
				"failed to create intermediary file for HEIF format: %w",
				err,
			)
		}

		// Ensure intermediary file is cleaned up
		defer func() {
			os.Remove(intermediaryFileAbsPath)
		}()

		// Update meta to point to intermediary file for the rest of the
		// generation process
		meta.OrigFilesRootDir = meta.ThumbFileAbsDir
		meta.OrigFileRelPath = filepath.Base(intermediaryFileAbsPath)
	}

	startTime := time.Now()
	origFileAbsPath := mkOriginalFileAbsPath(meta)

	// Load original file into memory
	origFileBytes, err := os.ReadFile(origFileAbsPath)
	if err != nil {
		return fmt.Errorf(
			"failed to read original file: %w",
			err)
	}

	// Get original image dimensions
	origDimensions, err := g.dimensions(origFileBytes)
	if err != nil {
		return err
	}

	// Generate thumbnails for each target width
	for _, targetWidth := range meta.ThumbWidths {
		select {
		case <-ctx.Done():
			slog.Warn(
				"Context cancelled during thumbs generation",
			)
			return ctx.Err()
		default:
		}

		if err := g.generateThumb(
			meta,
			origFileBytes,
			origDimensions,
			targetWidth,
		); err != nil {
			return err
		}
	}

	g.telemetry.Metrics().Duration(
		metrics.LilptThumbGenDuration,
		time.Since(startTime),
	)
	return nil
}

// Lilliput doesn't support HEIC format, so we convert it to JPEG first and
// then create thumbnails from the converted file.
func (g *ImageThumbsGenerator) mkIntermediaryFile(
	ctx context.Context,
	meta ThumbnailMeta,
) (string, error) {
	origFileAbsPath := mkOriginalFileAbsPath(meta)
	intermediaryFileAbsPath := mkIntermediaryThumbFileAbsPath(meta, ".jpg")

	err := g.formatConverter.ConvertWithoutFormatsCheck(
		ctx,
		origFileAbsPath,
		intermediaryFileAbsPath,
		format.JPEG,
	)

	if err != nil {
		// Clean up possible corrupted intermediary file
		os.Remove(intermediaryFileAbsPath)
		return "", err
	}

	return intermediaryFileAbsPath, nil
}

func (g *ImageThumbsGenerator) generateThumb(
	meta ThumbnailMeta,
	origFileBytes []byte,
	origFileDimensions *ImgDimensions,
	targetWidth int,
) error {

	maxImageAxis := max(origFileDimensions.Width, origFileDimensions.Height)
	if maxImageAxis > g.maxAllowedImageDimension {
		g.telemetry.Metrics().Increment(
			metrics.ImageThumbGeneratorMaxImageDimensionExceeded,
		)
		return &MaxImgDimensionExceededError{
			Width:               origFileDimensions.Width,
			Height:              origFileDimensions.Height,
			MaxAllowedDimension: g.maxAllowedImageDimension,
		}
	}

	decoder, err := g.decode(origFileBytes)
	if err != nil {
		return err
	}
	defer decoder.Close()

	// Compute target height to maintain aspect ratio
	targetHeight := (origFileDimensions.Height * targetWidth) / origFileDimensions.Width

	imgOpts := &lilliput.ImageOptions{
		FileType:              ThumbsExtension,
		Width:                 targetWidth,
		Height:                targetHeight,
		ResizeMethod:          lilliput.ImageOpsFit,
		NormalizeOrientation:  true,
		EncodeOptions:         g.encodeOptionsByExtension(ThumbsExtension),
		DisableAnimatedOutput: true,
		EncodeTimeout:         5 * time.Second,
	}
	resizedImgBuf, err := g.lpResourceManager.Transform(
		decoder,
		imgOpts,
		maxImageAxis,
	)
	if err != nil {
		if errors.Is(err, lilliput.ErrBufTooSmall) {
			g.telemetry.Metrics().Increment(
				metrics.LPErrOutputBufferTooSmall,
			)
		}

		return fmt.Errorf("failed to create thumbnail: %w", err)
	}

	thumbFileAbsPath := mkThumbFileAbsPath(meta, targetWidth, ThumbsExtension)
	if err := os.WriteFile(thumbFileAbsPath, resizedImgBuf, 0644); err != nil {
		return fmt.Errorf(
			"failed to write thumbnail file %s: %w",
			thumbFileAbsPath,
			err)
	}

	g.telemetry.Metrics().Increment(metrics.ThumbCreated)
	return nil
}

func (g *ImageThumbsGenerator) encodeOptionsByExtension(
	extension string,
) map[int]int {

	// Select encoder options based on output file format.
	// Different formats expect different option keys in lilliput.
	switch strings.ToLower(extension) {
	case ".webp":
		// WebP uses a quality value from 0-100.
		// Higher values = better visual quality and larger file size.
		return map[int]int{lilliput.WebpQuality: ThumbsQuality}
	case ".png":
		// PNG uses compression level from 0-9 (lossless format).
		// Higher values usually reduce size but may take more CPU time.
		return map[int]int{lilliput.PngCompression: 3}
	default:
		// Default to JPEG quality (0-100).
		// Higher values = better quality and larger file size.
		return map[int]int{lilliput.JpegQuality: ThumbsQuality}
	}
}

// dimensions retrieves the width and height in pixels by reading the
// image header via lilliput decoder, without fully decoding the image.
func (g *ImageThumbsGenerator) dimensions(
	fileBytes []byte,
) (*ImgDimensions, error) {
	decoder, err := g.decode(fileBytes)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	imgHeader, err := decoder.Header()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read image header: %w",
			err,
		)
	}

	imgDimensions := &ImgDimensions{
		Width:  imgHeader.Width(),
		Height: imgHeader.Height(),
	}

	return imgDimensions, nil
}

func (g *ImageThumbsGenerator) decode(
	fileBytes []byte,
) (lilliput.Decoder, error) {
	decoder, err := lilliput.NewDecoder(fileBytes)

	if err != nil {
		return nil, fmt.Errorf(
			"lilliput NewDecoder error: %w",
			err,
		)
	}

	return decoder, nil
}

func (g *ImageThumbsGenerator) isOriginalFileFormatSupported(
	originalFileFormat format.Format,
) error {
	supportedFormats := []format.Format{
		format.JPEG, format.PNG, format.WEBP, format.HEIF,
	}

	if !slices.Contains(supportedFormats, originalFileFormat) {
		return fmt.Errorf(
			"unsupported original file format: %v",
			originalFileFormat,
		)
	}

	return nil
}
