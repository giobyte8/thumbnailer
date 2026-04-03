package thumbsgen

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	frameextractor "github.com/giobyte8/thumbnailer/internal/thumbs_gen/frame_extractor"
)

type RoutedThumbsGenerator struct {
	formatDetector *format.FormatDetector
	routes         map[format.Format]ThumbsGenerator
}

func NewRoutedThumbsGenerator(
	telemetryService *telemetry.TelemetrySvc,
) *RoutedThumbsGenerator {
	formatDetector := format.NewFormatDetector()
	formatConverter := format.NewFormatConverter(formatDetector)
	frameExtractor := frameextractor.NewFrameExtractor(formatDetector)

	imageThumbsGenerator := NewImageThumbsGenerator(
		telemetryService,
		formatConverter,
		formatDetector,
	)

	videoThumbsGenerator := NewVideoThumbsGenerator(
		frameExtractor,
		imageThumbsGenerator,
	)

	routes := map[format.Format]ThumbsGenerator{
		format.JPEG: imageThumbsGenerator,
		format.PNG:  imageThumbsGenerator,
		format.WEBP: imageThumbsGenerator,
		format.HEIF: imageThumbsGenerator,

		format.MOV: videoThumbsGenerator,
		format.MP4: videoThumbsGenerator,
		format.M4V: videoThumbsGenerator,
	}

	return &RoutedThumbsGenerator{
		formatDetector: formatDetector,
		routes:         routes,
	}
}

// Generate dispatches thumbnail generation to appropriate generator
// based on the original file format.
func (g *RoutedThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	origFileAbsPath := mkOriginalFileAbsPath(meta)

	format, err := g.formatDetector.Detect(origFileAbsPath)
	if err != nil {
		return fmt.Errorf(
			"failed to detect format for file %s: %w",
			meta.OrigFileRelPath,
			err)
	}

	return g.GenerateWithoutFormatsCheck(ctx, meta, format)
}

// GenerateWithoutFormatsCheck implements ThumbsGenerator.
func (g *RoutedThumbsGenerator) GenerateWithoutFormatsCheck(
	ctx context.Context,
	meta ThumbnailMeta,
	origFileFormat format.Format,
) error {
	generator, found := g.routes[origFileFormat]
	if !found {
		slog.Warn(
			"Discarding unsupported file format for thumbnail generation",
			"filePath",
			meta.OrigFileRelPath,
			"format",
			origFileFormat,
		)
		return nil
	}

	return generator.GenerateWithoutFormatsCheck(ctx, meta, origFileFormat)
}
