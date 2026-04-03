package thumbsgen

import (
	"context"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/testutils"
	frameextractor "github.com/giobyte8/thumbnailer/internal/thumbs_gen/frame_extractor"
	_ "golang.org/x/image/webp"
)

func TestVideoThumbsGenerator_Integration_SupportedCases(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	origFilesRootDir := testutils.TestFilesDir()
	generator := mkVideoGenerator(t)

	tests := []struct {
		name         string
		originalFile string
		thumbWidths  []int
	}{
		{
			name:         "mov source generates three thumbnails",
			originalFile: "10 lake_hdr.mov",
			thumbWidths:  []int{120, 240, 360},
		},
		{
			name:         "mp4 source generates three thumbnails",
			originalFile: "11 whatsapp.mp4",
			thumbWidths:  []int{100, 200, 300},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := ThumbnailMeta{
				OrigFilesRootDir: origFilesRootDir,
				OrigFileRelPath:  tc.originalFile,
				ThumbFileAbsDir:  t.TempDir(),
				ThumbWidths:      tc.thumbWidths,
			}

			if err := generator.Generate(context.Background(), meta); err != nil {
				t.Fatalf("generate failed: %v", err)
			}

			for _, width := range tc.thumbWidths {
				thumbAbsPath := mkThumbFileAbsPath(meta, width, ThumbsExtension)
				assertVideoThumbnailCreated(t, thumbAbsPath, width)
			}
		})
	}
}

func TestVideoThumbsGenerator_Integration_WithoutFormatsCheck(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	origFilesRootDir := testutils.TestFilesDir()
	generator := mkVideoGenerator(t)

	tests := []struct {
		name           string
		originalFile   string
		originalFormat format.Format
		thumbWidths    []int
	}{
		{
			name:           "mov source generates thumbnails without format checks",
			originalFile:   "10 lake_hdr.mov",
			originalFormat: format.MOV,
			thumbWidths:    []int{120, 240},
		},
		{
			name:           "mp4 source generates thumbnails without format checks",
			originalFile:   "11 whatsapp.mp4",
			originalFormat: format.MP4,
			thumbWidths:    []int{100, 200},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := ThumbnailMeta{
				OrigFilesRootDir: origFilesRootDir,
				OrigFileRelPath:  tc.originalFile,
				ThumbFileAbsDir:  t.TempDir(),
				ThumbWidths:      tc.thumbWidths,
			}

			if err := generator.GenerateWithoutFormatsCheck(
				context.Background(),
				meta,
				tc.originalFormat,
			); err != nil {
				t.Fatalf("generate without formats check failed: %v", err)
			}

			for _, width := range tc.thumbWidths {
				thumbAbsPath := mkThumbFileAbsPath(meta, width, ThumbsExtension)
				assertVideoThumbnailCreated(t, thumbAbsPath, width)
			}
		})
	}
}

func mkVideoGenerator(t *testing.T) *VideoThumbsGenerator {
	t.Helper()
	t.Setenv("OTEL_ENABLED", "false")

	telemetrySvc, err := telemetry.NewTelemetrySvc(context.Background())
	if err != nil {
		t.Fatalf("failed to init telemetry service: %v", err)
	}
	t.Cleanup(func() {
		_ = telemetrySvc.Shutdown(context.Background())
	})

	fmtDetector := format.NewFormatDetector()
	frameExtractor := frameextractor.NewFrameExtractor(fmtDetector)
	imageGenerator := NewImageThumbsGenerator(
		telemetrySvc,
		format.NewFormatConverter(fmtDetector),
		fmtDetector,
	)

	return NewVideoThumbsGenerator(
		frameExtractor,
		imageGenerator,
	)
}

func assertVideoThumbnailCreated(t *testing.T, thumbAbsPath string, expectedWidth int) {
	t.Helper()

	info, err := os.Stat(thumbAbsPath)
	if err != nil {
		t.Fatalf("expected thumbnail file to exist %s: %v", thumbAbsPath, err)
	}

	if info.Size() <= 0 {
		t.Fatalf("expected non-empty thumbnail file %s", thumbAbsPath)
	}

	if filepath.Ext(thumbAbsPath) != ThumbsExtension {
		t.Fatalf("unexpected thumbnail extension for %s", thumbAbsPath)
	}

	fileHandle, err := os.Open(thumbAbsPath)
	if err != nil {
		t.Fatalf("failed to open thumbnail %s: %v", thumbAbsPath, err)
	}
	defer fileHandle.Close()

	config, _, err := image.DecodeConfig(fileHandle)
	if err != nil {
		t.Fatalf("failed to decode thumbnail %s: %v", thumbAbsPath, err)
	}

	if config.Width != expectedWidth {
		t.Fatalf(
			"unexpected width for %s: got %d want %d",
			thumbAbsPath,
			config.Width,
			expectedWidth,
		)
	}

	if config.Height <= 0 {
		t.Fatalf("invalid height for %s: %d", thumbAbsPath, config.Height)
	}
}
