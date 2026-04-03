package frameextractor

import (
	"context"
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
)

func TestExtractor_UnsupportedCases(t *testing.T) {
	tests := []struct {
		name        string
		fromAbsPath string
		intoAbsPath string
		errContains string
	}{
		{
			name:        "unsupported destination extension",
			fromAbsPath: "/tmp/in.mov",
			intoAbsPath: "/tmp/out.webp",
			errContains: "unsupported destination file extension",
		},
		{
			name:        "unsupported source format",
			fromAbsPath: testutils.TestFilePath("7 flower.webp"),
			intoAbsPath: "/tmp/out.jpg",
			errContains: "unsupported source format",
		},
	}

	extractor := NewFrameExtractor(mkTestTelemetrySvc(t), format.NewFormatDetector())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := extractor.Extract(
				context.Background(),
				tc.fromAbsPath,
				tc.intoAbsPath,
			)

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}

			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}

func TestExtractor_Integration_SupportedCases(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	tests := []struct {
		name        string
		fromAbsPath string
		intoAbsPath string
	}{
		{
			name:        "extract frame from mov",
			fromAbsPath: testutils.TestFilePath("10 lake_hdr.mov"),
			intoAbsPath: filepath.Join(t.TempDir(), "frame.jpg"),
		},
		{
			name:        "extract frame from mp4",
			fromAbsPath: testutils.TestFilePath("11 whatsapp.mp4"),
			intoAbsPath: filepath.Join(t.TempDir(), "frame.jpg"),
		},
	}

	extractor := NewFrameExtractor(mkTestTelemetrySvc(t), format.NewFormatDetector())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fromAbsPath := tc.fromAbsPath
			intoAbsPath := tc.intoAbsPath

			err := extractor.Extract(context.Background(), fromAbsPath, intoAbsPath)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			fileInfo, err := os.Stat(intoAbsPath)
			if err != nil {
				t.Fatalf("expected output file to exist: %v", err)
			}

			if fileInfo.Size() <= 0 {
				t.Fatalf("expected non-empty output file")
			}

			fileHandle, err := os.Open(intoAbsPath)
			if err != nil {
				t.Fatalf("failed to open output image: %v", err)
			}
			defer fileHandle.Close()

			config, _, err := image.DecodeConfig(fileHandle)
			if err != nil {
				t.Fatalf("failed to decode output image: %v", err)
			}

			if config.Width <= 0 || config.Height <= 0 {
				t.Fatalf(
					"invalid extracted image dimensions: %dx%d",
					config.Width,
					config.Height,
				)
			}
		})
	}
}

func mkTestTelemetrySvc(t *testing.T) *telemetry.TelemetrySvc {
	t.Helper()
	t.Setenv("OTEL_ENABLED", "false")

	telemetrySvc, err := telemetry.NewTelemetrySvc(context.Background())
	if err != nil {
		t.Fatalf("failed to init telemetry service: %v", err)
	}

	t.Cleanup(func() {
		_ = telemetrySvc.Shutdown(context.Background())
	})

	return telemetrySvc
}
