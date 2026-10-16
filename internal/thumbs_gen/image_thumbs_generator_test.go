package thumbsgen

import (
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/testutils"
	_ "golang.org/x/image/webp"
)

func TestImageThumbsGenerator_Integration_SupportedCases(t *testing.T) {
	generator := mkGenerator(t)

	origFilesRootDir := testutils.TestFilesDir()

	tests := []struct {
		name         string
		originalFile string
		thumbWidths  []int
	}{
		{
			name:         "jpg source generates two thumbnails",
			originalFile: "1 house.jpg",
			thumbWidths:  []int{120, 240},
		},
		{
			name:         "png source generates two thumbnails",
			originalFile: "3 screenshot.png",
			thumbWidths:  []int{100, 200},
		},
		{
			name:         "webp source generates two thumbnails",
			originalFile: "7 flower.webp",
			thumbWidths:  []int{96, 192},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			thumbsDir := t.TempDir()

			meta := ThumbnailMeta{
				OrigFilesRootDir: origFilesRootDir,
				OrigFileRelPath:  tc.originalFile,
				ThumbFileAbsDir:  thumbsDir,
				ThumbWidths:      tc.thumbWidths,
			}

			if err := generator.Generate(context.Background(), meta); err != nil {
				t.Fatalf("generate failed: %v", err)
			}

			for _, width := range tc.thumbWidths {
				thumbAbsPath := mkThumbFileAbsPath(meta, width, ThumbsExtension)
				assertThumbnailCreated(t, thumbAbsPath, width)
			}
		})
	}
}

func TestImageThumbsGenerator_AtMaximumDimension(t *testing.T) {
	originalsDir := testutils.TestFilesDir()
	originalPath := filepath.Join(originalsDir, "1 house.jpg")
	file, err := os.Open(originalPath)
	if err != nil {
		t.Fatalf("failed to open original image: %v", err)
	}
	imageConfig, _, err := image.DecodeConfig(file)
	file.Close()
	if err != nil {
		t.Fatalf("failed to read original image dimensions: %v", err)
	}
	maxDimension := max(imageConfig.Width, imageConfig.Height)

	generator := mkGenerator(t, maxDimension)
	meta := ThumbnailMeta{
		OrigFilesRootDir: originalsDir,
		OrigFileRelPath:  "1 house.jpg",
		ThumbFileAbsDir:  t.TempDir(),
		ThumbWidths:      []int{120},
	}

	if err := generator.Generate(context.Background(), meta); err != nil {
		t.Fatalf("image at the maximum dimension should be accepted: %v", err)
	}
}

func TestImageThumbsGenerator_RejectsAboveMaximumDimension(t *testing.T) {
	originalsDir := testutils.TestFilesDir()
	originalPath := filepath.Join(originalsDir, "1 house.jpg")
	file, err := os.Open(originalPath)
	if err != nil {
		t.Fatalf("failed to open original image: %v", err)
	}
	imageConfig, _, err := image.DecodeConfig(file)
	file.Close()
	if err != nil {
		t.Fatalf("failed to read original image dimensions: %v", err)
	}
	maxDimension := max(imageConfig.Width, imageConfig.Height)

	generator := mkGenerator(t, maxDimension-1)
	meta := ThumbnailMeta{
		OrigFilesRootDir: originalsDir,
		OrigFileRelPath:  "1 house.jpg",
		ThumbFileAbsDir:  t.TempDir(),
		ThumbWidths:      []int{120},
	}

	err = generator.Generate(context.Background(), meta)
	var dimensionErr *MaxImgDimensionExceededError
	if !errors.As(err, &dimensionErr) {
		t.Fatalf("expected MaxImgDimensionExceededError, got %v", err)
	}
	if dimensionErr.Width != imageConfig.Width || dimensionErr.Height != imageConfig.Height {
		t.Fatalf("dimension error = %+v, want source dimensions %dx%d", dimensionErr, imageConfig.Width, imageConfig.Height)
	}
	if dimensionErr.MaxAllowedDimension != maxDimension-1 {
		t.Fatalf("maximum dimension = %d, want %d", dimensionErr.MaxAllowedDimension, maxDimension-1)
	}
}

func TestImageThumbsGenerator_Integration_HEIF(t *testing.T) {
	if _, err := exec.LookPath("heif-convert"); err != nil {
		t.Skip("heif-convert not available, skipping HEIF test")
	}

	generator := mkGenerator(t)

	meta := ThumbnailMeta{
		OrigFilesRootDir: testutils.TestFilesDir(),
		OrigFileRelPath:  "6 gastown_heif.jpg",
		ThumbFileAbsDir:  t.TempDir(),
		ThumbWidths:      []int{120, 240},
	}

	if err := generator.Generate(context.Background(), meta); err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	for _, width := range meta.ThumbWidths {
		thumbAbsPath := mkThumbFileAbsPath(meta, width, ThumbsExtension)
		assertThumbnailCreated(t, thumbAbsPath, width)
	}

	intermediaryAbsPath := filepath.Join(meta.ThumbFileAbsDir, baseNameNoExt(meta)+".jpg")
	if _, err := os.Stat(intermediaryAbsPath); !os.IsNotExist(err) {
		t.Fatalf("expected intermediary converted file to be removed, stat error: %v", err)
	}
}

func TestImageThumbsGenerator_UnsupportedMedia(t *testing.T) {
	generator := mkGenerator(t)

	meta := ThumbnailMeta{
		OrigFilesRootDir: testutils.TestFilesDir(),
		OrigFileRelPath:  "10 lake_hdr.mov",
		ThumbFileAbsDir:  t.TempDir(),
		ThumbWidths:      []int{120, 240},
	}

	err := generator.Generate(context.Background(), meta)
	if err == nil {
		t.Fatalf("expected unsupported media error, got nil")
	}

	if !strings.Contains(err.Error(), "cannot generate thumbnails") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if !strings.Contains(err.Error(), "unsupported original file format: mov") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func mkGenerator(t *testing.T, maxDimension ...int) *ImageThumbsGenerator {
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
	configuredMaxDimension := 8192
	if len(maxDimension) > 0 {
		configuredMaxDimension = maxDimension[0]
	}
	return NewImageThumbsGenerator(
		telemetrySvc,
		format.NewFormatConverter(telemetrySvc, fmtDetector),
		fmtDetector,
		configuredMaxDimension,
	)
}

func assertThumbnailCreated(t *testing.T, thumbAbsPath string, expectedWidth int) {
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
