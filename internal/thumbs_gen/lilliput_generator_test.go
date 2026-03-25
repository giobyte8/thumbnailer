package thumbsgen

import (
	"context"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/giobyte8/thumbnailer/internal/telemetry"
	_ "golang.org/x/image/webp"
)

func TestLilliputGenerate_CreatesExpectedThumbnails(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "false")

	telemetrySvc, err := telemetry.NewTelemetrySvc(context.Background())
	if err != nil {
		t.Fatalf("failed to init telemetry service: %v", err)
	}
	t.Cleanup(func() {
		_ = telemetrySvc.Shutdown(context.Background())
	})

	generator := NewLilliputThumbsGenerator(telemetrySvc)

	fixturePath := fixtureImagePath(t)
	workDir := t.TempDir()

	origRoot := filepath.Join(workDir, "originals")
	thumbsDir := filepath.Join(workDir, "thumbs", "media")
	origRelPath := filepath.Join("media", "sample.png")
	origAbsPath := filepath.Join(origRoot, origRelPath)

	if err := os.MkdirAll(filepath.Dir(origAbsPath), 0o755); err != nil {
		t.Fatalf("failed to create originals dir: %v", err)
	}

	input, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	if err := os.WriteFile(origAbsPath, input, 0o644); err != nil {
		t.Fatalf("failed to write original file: %v", err)
	}

	meta := ThumbnailMeta{
		OrigFilesRootDir: origRoot,
		OrigFileRelPath:  origRelPath,
		ThumbFileAbsDir:  thumbsDir,
		ThumbWidths:      []int{16, 32},
	}

	if err := os.MkdirAll(meta.ThumbFileAbsDir, 0o755); err != nil {
		t.Fatalf("failed to create thumbs dir: %v", err)
	}

	if err := generator.Generate(context.Background(), meta); err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	assertThumbnailWidth(t, filepath.Join(thumbsDir, "sample_16px.webp"), 16)
	assertThumbnailWidth(t, filepath.Join(thumbsDir, "sample_32px.webp"), 32)
}

func fixtureImagePath(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}

	fixture := filepath.Join(filepath.Dir(thisFile), "testdata", "sample.png")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture not found at %s: %v", fixture, err)
	}

	return fixture
}

func assertThumbnailWidth(t *testing.T, thumbPath string, expectedWidth int) {
	t.Helper()

	f, err := os.Open(thumbPath)
	if err != nil {
		t.Fatalf("failed to open thumbnail %s: %v", thumbPath, err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("failed to decode thumbnail %s: %v", thumbPath, err)
	}

	if cfg.Width != expectedWidth {
		t.Fatalf("unexpected width for %s: got %d want %d", thumbPath, cfg.Width, expectedWidth)
	}

	if cfg.Height <= 0 {
		t.Fatalf("invalid height for %s: %d", thumbPath, cfg.Height)
	}
}
