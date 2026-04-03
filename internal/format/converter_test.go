package format

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/testutils"
)

func TestConverter_UnsupportedCases(t *testing.T) {
	tests := []struct {
		name        string
		srcPath     string
		dstPath     string
		format      Format
		errContains string
	}{
		{
			name:        "unsupported output extension",
			srcPath:     "/tmp/in.heic",
			dstPath:     "/tmp/out.webp",
			format:      JPEG,
			errContains: "unsupported destination file extension",
		},
		{
			name:        "unsupported output format",
			srcPath:     "/tmp/in.heic",
			dstPath:     "/tmp/out.jpg",
			format:      WEBP,
			errContains: "unsupported destination format",
		},
		{
			name:        "unsupported source format",
			srcPath:     testutils.TestFilePath("7 flower.webp"),
			dstPath:     "/tmp/out.jpg",
			format:      JPEG,
			errContains: "unsupported source format",
		},
	}

	converter := NewFormatConverter(mkTestTelemetrySvc(t), NewFormatDetector())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := converter.Convert(context.Background(), tc.srcPath, tc.dstPath, tc.format)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}

func TestConverter_Integration_SupportedCase(t *testing.T) {
	if _, err := exec.LookPath("heif-convert"); err != nil {
		t.Skip("heif-convert not available, skipping integration test")
	}

	srcPath := testutils.TestFilePath("4 thai_no_edits.heic")
	dstPath := filepath.Join(t.TempDir(), "converted.jpg")
	format := JPEG

	// Convert test image to JPEG
	converter := NewFormatConverter(mkTestTelemetrySvc(t), NewFormatDetector())
	err := converter.Convert(context.Background(), srcPath, dstPath, format)
	if err != nil {
		t.Fatalf(
			"unexpected error converting %q to %q: %v",
			srcPath,
			dstPath,
			err,
		)
	}

	// Validate output file exists and is not empty
	fileInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	if fileInfo.Size() <= 0 {
		t.Fatalf("expected non-empty output file")
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
