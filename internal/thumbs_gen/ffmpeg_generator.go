package thumbsgen

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

type FFmpegThumbsGenerator struct {
	telemetry  *telemetry.TelemetrySvc
	ffmpegPath string
}

func NewFFmpegThumbsGenerator(
	telemetry *telemetry.TelemetrySvc,
) *FFmpegThumbsGenerator {
	return &FFmpegThumbsGenerator{
		telemetry:  telemetry,
		ffmpegPath: "ffmpeg",
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

	for _, tgtWidth := range meta.ThumbWidths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		thumbFileAbsPath := mkThumbFileAbsPath(meta, tgtWidth, ThumbsExtension)
		cmd := exec.CommandContext(
			ctx,
			g.ffmpegPath,
			g.mkFFmpegArgs(origFileAbsPath, thumbFileAbsPath, tgtWidth)...,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"ffmpeg thumbnail generation failed for %s (%dpx): %w. output: %s",
				meta.OrigFileRelPath,
				tgtWidth,
				err,
				strings.TrimSpace(string(output)),
			)
		}

		slog.Debug(
			"FFmpeg thumbnail created",
			"path",
			thumbFileAbsPath,
			"width",
			tgtWidth,
		)

		if g.telemetry != nil {
			g.telemetry.Metrics().Increment(metrics.ThumbCreated)
		}
	}

	return nil
}

func (g *FFmpegThumbsGenerator) mkFFmpegArgs(
	inputFileAbsPath string,
	outputFileAbsPath string,
	width int,
) []string {
	return []string{
		"-y",
		"-ss", "00:00:01",
		"-i", inputFileAbsPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2", width),
		"-q:v", strconv.Itoa(ThumbsQuality),
		outputFileAbsPath,
	}
}

func isFFmpegSupported(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp4", ".mov":
		return true
	default:
		return false
	}
}
