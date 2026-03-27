package thumbsgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	frameextractor "github.com/giobyte8/thumbnailer/internal/thumbs_gen/frame_extractor"
)

type FFmpegThumbsGenerator struct {
	frameExtractor frameextractor.VideoFrameExtractor
	imageGenerator ThumbsGenerator
}

// NewFFmpegThumbsGenerator builds a video thumbnail generator with explicit dependencies.
func NewFFmpegThumbsGenerator(
	frameExtractor frameextractor.VideoFrameExtractor,
	imageGenerator ThumbsGenerator,
) *FFmpegThumbsGenerator {
	return &FFmpegThumbsGenerator{
		frameExtractor: frameExtractor,
		imageGenerator: imageGenerator,
	}
}

func (g *FFmpegThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	if !isFFmpegSupported(meta.OrigFileRelPath) {
		return fmt.Errorf(
			"unsupported file extension for ffmpeg generator: %s",
			filepath.Ext(meta.OrigFileRelPath),
		)
	}

	origFileAbsPath := filepath.Join(meta.OrigFilesRootDir, meta.OrigFileRelPath)
	frameAbsPath := mkDerivedFileAbsPath(meta, ".jpg")
	defer func() {
		_ = os.Remove(frameAbsPath)
	}()

	if err := g.frameExtractor.Extract(ctx, origFileAbsPath, frameAbsPath); err != nil {
		return fmt.Errorf(
			"failed to extract frame from video %s: %w",
			meta.OrigFileRelPath,
			err,
		)
	}

	frameMeta := meta
	frameMeta.OrigFilesRootDir = meta.ThumbFileAbsDir
	frameMeta.OrigFileRelPath = filepath.Base(frameAbsPath)
	if err := g.imageGenerator.Generate(ctx, frameMeta); err != nil {
		return fmt.Errorf(
			"failed to generate video thumbnails from extracted frame for %s: %w",
			meta.OrigFileRelPath,
			err,
		)
	}

	return nil
}

func isFFmpegSupported(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp4", ".mov":
		return true
	default:
		return false
	}
}
