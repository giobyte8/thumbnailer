package thumbsgen

import (
	"context"
	"errors"
	"testing"
)

type recordingGenerator struct {
	calledWith []ThumbnailMeta
	err        error
}

func (g *recordingGenerator) Generate(
	_ context.Context,
	meta ThumbnailMeta,
) error {
	g.calledWith = append(g.calledWith, meta)
	return g.err
}

func TestRoutedThumbsGenerator_DelegatesByExtension(t *testing.T) {
	videoGen := &recordingGenerator{}
	imageGen := &recordingGenerator{}
	generator := NewDefaultRoutedThumbsGenerator(videoGen, imageGen)

	err := generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "movie.MP4"})
	if err != nil {
		t.Fatalf("unexpected error for mp4 route: %v", err)
	}

	err = generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "image.JPEG"})
	if err != nil {
		t.Fatalf("unexpected error for jpeg route: %v", err)
	}

	err = generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "clip.mov"})
	if err != nil {
		t.Fatalf("unexpected error for mov route: %v", err)
	}

	err = generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "photo.heic"})
	if err != nil {
		t.Fatalf("unexpected error for heic route: %v", err)
	}

	if len(videoGen.calledWith) != 2 {
		t.Fatalf("unexpected video generator call count: got %d want %d", len(videoGen.calledWith), 2)
	}

	if len(imageGen.calledWith) != 2 {
		t.Fatalf("unexpected image generator call count: got %d want %d", len(imageGen.calledWith), 2)
	}
}

func TestRoutedThumbsGenerator_UnsupportedExtensionIsDiscarded(t *testing.T) {
	videoGen := &recordingGenerator{}
	imageGen := &recordingGenerator{}
	generator := NewDefaultRoutedThumbsGenerator(videoGen, imageGen)

	err := generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "notes.txt"})
	if err != nil {
		t.Fatalf("unsupported extension should be discarded without error, got: %v", err)
	}

	if len(videoGen.calledWith) != 0 {
		t.Fatalf("video generator should not be called for unsupported extension")
	}

	if len(imageGen.calledWith) != 0 {
		t.Fatalf("image generator should not be called for unsupported extension")
	}
}

func TestRoutedThumbsGenerator_PropagatesDelegatedError(t *testing.T) {
	wantErr := errors.New("boom")
	videoGen := &recordingGenerator{err: wantErr}
	imageGen := &recordingGenerator{}
	generator := NewDefaultRoutedThumbsGenerator(videoGen, imageGen)

	err := generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "video.mp4"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected delegated error: got %v want %v", err, wantErr)
	}
}
