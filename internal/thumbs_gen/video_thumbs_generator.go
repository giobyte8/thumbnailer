package thumbsgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giobyte8/thumbnailer/internal/format"
	frameextractor "github.com/giobyte8/thumbnailer/internal/thumbs_gen/frame_extractor"
)

type VideoThumbsGenerator struct {
	frameExtractor       *frameextractor.Extractor
	imageThumbsGenerator ThumbsGenerator
}

// NewVideoThumbsGenerator builds a video thumbnail generator
// with explicit dependencies.
func NewVideoThumbsGenerator(
	frameExtractor *frameextractor.Extractor,
	imageThumbsGenerator ThumbsGenerator,
) *VideoThumbsGenerator {
	return &VideoThumbsGenerator{
		frameExtractor:       frameExtractor,
		imageThumbsGenerator: imageThumbsGenerator,
	}
}

// Generate implements ThumbsGenerator.
func (g *VideoThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	return g.generateThumb(ctx, meta, true)
}

// GenerateWithoutFormatsCheck implements ThumbsGenerator.
func (g *VideoThumbsGenerator) GenerateWithoutFormatsCheck(
	ctx context.Context,
	meta ThumbnailMeta,
	origFileFormat format.Format,
) error {
	return g.generateThumb(ctx, meta, false)
}

func (g *VideoThumbsGenerator) generateThumb(
	ctx context.Context,
	meta ThumbnailMeta,
	withFormatChecks bool,
) error {
	origFileAbsPath := mkOriginalFileAbsPath(meta)
	vidFrameAbsPath := mkIntermediaryThumbFileAbsPath(meta, ".jpg")
	defer func() {
		_ = os.Remove(vidFrameAbsPath)
	}()

	// Extract a frame from the video
	var err error
	if withFormatChecks {
		err = g.frameExtractor.Extract(ctx, origFileAbsPath, vidFrameAbsPath)
	} else {
		err = g.frameExtractor.ExtractWithoutFormatsCheck(ctx, origFileAbsPath, vidFrameAbsPath)
	}
	if err != nil {
		return fmt.Errorf(
			"failed to extract frame from video %s: %w",
			meta.OrigFileRelPath,
			err,
		)
	}

	// Replace original file info with intermediary frame file
	frameMeta := meta
	frameMeta.OrigFilesRootDir = meta.ThumbFileAbsDir
	frameMeta.OrigFileRelPath = filepath.Base(vidFrameAbsPath)

	// Generate thumbnails from the extracted frame
	if withFormatChecks {
		err = g.imageThumbsGenerator.Generate(ctx, frameMeta)
	} else {
		err = g.imageThumbsGenerator.GenerateWithoutFormatsCheck(
			ctx,
			frameMeta,
			format.JPEG,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"failed to generate video thumbnails from extracted frame for %s: %w",
			meta.OrigFileRelPath,
			err,
		)
	}

	return nil
}
