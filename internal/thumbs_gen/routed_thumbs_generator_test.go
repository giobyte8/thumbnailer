package thumbsgen

import (
	"context"
	"errors"
	"testing"

	"github.com/giobyte8/thumbnailer/internal/format"
	"github.com/giobyte8/thumbnailer/internal/testutils"
)

type recordingThumbsGenerator struct {
	calledGenerate              int
	calledGenerateWithoutChecks int
	lastMeta                    ThumbnailMeta
	lastOriginalFileFormat      format.Format
	err                         error
}

func (g *recordingThumbsGenerator) Generate(
	_ context.Context,
	meta ThumbnailMeta,
) error {
	g.calledGenerate++
	g.lastMeta = meta
	return g.err
}

func (g *recordingThumbsGenerator) GenerateWithoutFormatsCheck(
	_ context.Context,
	meta ThumbnailMeta,
	origFileFormat format.Format,
) error {
	g.calledGenerateWithoutChecks++
	g.lastMeta = meta
	g.lastOriginalFileFormat = origFileFormat
	return g.err
}

func TestNewRoutedThumbsGenerator_DefaultRoutes(t *testing.T) {
	generator := NewRoutedThumbsGenerator(nil)

	if generator.routes[format.JPEG] == nil {
		t.Fatalf("expected JPEG route to be configured")
	}
	if generator.routes[format.PNG] == nil {
		t.Fatalf("expected PNG route to be configured")
	}
	if generator.routes[format.WEBP] == nil {
		t.Fatalf("expected WEBP route to be configured")
	}
	if generator.routes[format.HEIF] == nil {
		t.Fatalf("expected HEIF route to be configured")
	}
	if generator.routes[format.MOV] == nil {
		t.Fatalf("expected MOV route to be configured")
	}
	if generator.routes[format.MP4] == nil {
		t.Fatalf("expected MP4 route to be configured")
	}
	if generator.routes[format.M4V] == nil {
		t.Fatalf("expected M4V route to be configured")
	}
}

func TestRoutedThumbsGenerator_Generate_DetectsAndDelegates(t *testing.T) {
	recorder := &recordingThumbsGenerator{}
	generator := &RoutedThumbsGenerator{
		formatDetector: format.NewFormatDetector(),
		routes: map[format.Format]ThumbsGenerator{
			format.JPEG: recorder,
		},
	}

	meta := ThumbnailMeta{
		OrigFilesRootDir: testutils.TestFilesDir(),
		OrigFileRelPath:  "1 house.jpg",
	}

	if err := generator.Generate(context.Background(), meta); err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if recorder.calledGenerateWithoutChecks != 1 {
		t.Fatalf(
			"expected one delegated call, got %d",
			recorder.calledGenerateWithoutChecks,
		)
	}

	if recorder.lastOriginalFileFormat != format.JPEG {
		t.Fatalf(
			"unexpected delegated format: got %v want %v",
			recorder.lastOriginalFileFormat,
			format.JPEG,
		)
	}
}

func TestRoutedThumbsGenerator_GenerateWithoutFormatsCheck_UnsupportedFormatIsDiscarded(t *testing.T) {
	recorder := &recordingThumbsGenerator{}
	generator := &RoutedThumbsGenerator{
		formatDetector: format.NewFormatDetector(),
		routes: map[format.Format]ThumbsGenerator{
			format.JPEG: recorder,
		},
	}

	err := generator.GenerateWithoutFormatsCheck(
		context.Background(),
		ThumbnailMeta{OrigFileRelPath: "file.bin"},
		format.UNSUPPORTED,
	)
	if err != nil {
		t.Fatalf("unsupported format should be discarded without error: %v", err)
	}

	if recorder.calledGenerateWithoutChecks != 0 {
		t.Fatalf("expected no delegated calls for unsupported format")
	}
}

func TestRoutedThumbsGenerator_GenerateWithoutFormatsCheck_PropagatesDelegatedError(t *testing.T) {
	wantErr := errors.New("boom")
	recorder := &recordingThumbsGenerator{err: wantErr}
	generator := &RoutedThumbsGenerator{
		formatDetector: format.NewFormatDetector(),
		routes: map[format.Format]ThumbsGenerator{
			format.MOV: recorder,
		},
	}

	err := generator.GenerateWithoutFormatsCheck(
		context.Background(),
		ThumbnailMeta{OrigFileRelPath: "video.mov"},
		format.MOV,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected delegated error: got %v want %v", err, wantErr)
	}
}
