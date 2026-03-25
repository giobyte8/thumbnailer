package thumbsgen

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/discord/lilliput"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

type LilliputThumbsGenerator struct {
	telemetry *telemetry.TelemetrySvc
}

func NewLilliputThumbsGenerator(
	telemetry *telemetry.TelemetrySvc,
) *LilliputThumbsGenerator {

	return &LilliputThumbsGenerator{
		telemetry: telemetry,
	}
}

func (g *LilliputThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	slog.Debug(
		"Generating thumbnails",
		"origFile",
		meta.OrigFileRelPath,
	)

	origFileAbsPath := filepath.Join(
		meta.OrigFilesRootDir,
		meta.OrigFileRelPath,
	)

	// Load original file into memory
	inputBuf, err := g.readFile(origFileAbsPath)
	if err != nil {
		return err
	}

	// Using lilliput, Decode original image to retrieve its dimensions
	decoder, err := g.decode(meta.OrigFileRelPath, inputBuf)
	if err != nil {
		return err
	}
	defer decoder.Close()

	// Get original image dimensions
	origWidth, origHeight, err := g.getOrigDimensions(
		meta.OrigFileRelPath,
		decoder,
	)
	if err != nil {
		return err
	}

	// TODO: Determine appropriate 'maxSize' value
	// Set max resize buffer size
	ops := lilliput.NewImageOps(int(float64(origWidth) * 1.5))
	defer ops.Close()

	// TODO: Determine appropriate 'resizeBuffer' size
	// Create a 50MB buffer to store resized image(s)
	resizeBuffer := make([]byte, 500*1024*1024)

	// Iterate meta.TargetWidths and generate a thumbnail for each width
	for _, tgtWidth := range meta.ThumbWidths {
		select {
		case <-ctx.Done():
			slog.Warn(
				"Context cancelled during thumbs generation",
			)
			return ctx.Err()
		default:
			// Continue processing
		}

		// TODO: Find a way to reuse decoder for multiple widths
		decoder, err := g.decode(origFileAbsPath, inputBuf)
		if err != nil {
			return err
		}
		defer decoder.Close()

		tgtHeight := (origHeight * tgtWidth) / origWidth
		opts := &lilliput.ImageOptions{
			FileType:              ThumbsExtension,
			Width:                 tgtWidth,
			Height:                tgtHeight,
			ResizeMethod:          lilliput.ImageOpsFit,
			NormalizeOrientation:  true,
			EncodeOptions:         encodeOptionsByExtension(ThumbsExtension),
			DisableAnimatedOutput: true,
			EncodeTimeout:         5 * time.Second,
		}

		// Create thumbnail
		resizedImgBuf, err := ops.Transform(decoder, opts, resizeBuffer)
		if err != nil {
			return fmt.Errorf(
				"failed to create thumbnail for %s: %w",
				meta.OrigFileRelPath,
				err,
			)
		}

		thumbFileAbsPath := mkThumbFileAbsPath(meta, tgtWidth, ThumbsExtension)

		// Save thumbnail to file
		if err := os.WriteFile(thumbFileAbsPath, resizedImgBuf, 0644); err != nil {
			return fmt.Errorf(
				"failed to write thumbnail file %s: %w",
				thumbFileAbsPath,
				err,
			)
		}

		// Record metric to count generated thumbnail
		g.telemetry.Metrics().Increment(metrics.ThumbCreated)
	}

	return nil
}

func encodeOptionsByExtension(extension string) map[int]int {
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

func (g *LilliputThumbsGenerator) readFile(
	fileAbsPath string,
) ([]byte, error) {
	inputBuf, err := os.ReadFile(fileAbsPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read original file %s: %w",
			fileAbsPath,
			err,
		)
	}

	return inputBuf, nil
}

func (g *LilliputThumbsGenerator) decode(
	fileRelPath string,
	inputBuf []byte,
) (lilliput.Decoder, error) {
	decoder, err := lilliput.NewDecoder(inputBuf)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create lilliput decoder for %s: %w",
			fileRelPath,
			err,
		)
	}

	return decoder, nil
}

func (g *LilliputThumbsGenerator) getOrigDimensions(
	fileRelPath string,
	decoder lilliput.Decoder,
) (int, int, error) {
	imgHeader, err := decoder.Header()
	if err != nil {
		return 0, 0, fmt.Errorf(
			"failed to get image header for %s: %w",
			fileRelPath,
			err,
		)
	}

	origWidth := imgHeader.Width()
	origHeight := imgHeader.Height()
	if origWidth == 0 || origHeight == 0 {
		return 0, 0, fmt.Errorf(
			"invalid original image dimensions: width=%d, height=%d",
			origWidth,
			origHeight,
		)
	}

	return origWidth, origHeight, nil
}
