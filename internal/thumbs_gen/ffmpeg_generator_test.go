package thumbsgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeVideoFrameExtractor struct {
	calledFrom string
	calledInto string
	callCount  int
	err        error
}

func (e *fakeVideoFrameExtractor) Extract(
	_ context.Context,
	fromAbsPath string,
	intoAbsPath string,
) error {
	e.callCount++
	e.calledFrom = fromAbsPath
	e.calledInto = intoAbsPath

	if e.err != nil {
		return e.err
	}

	return os.WriteFile(intoAbsPath, []byte("frame"), 0o644)
}

type fakeImageThumbsGenerator struct {
	calledMeta ThumbnailMeta
	callCount  int
	err        error
}

func (g *fakeImageThumbsGenerator) Generate(
	_ context.Context,
	meta ThumbnailMeta,
) error {
	g.callCount++
	g.calledMeta = meta
	return g.err
}

func TestIsFFmpegSupported(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{name: "mp4", filePath: "video.mp4", want: true},
		{name: "mov upper", filePath: "video.MOV", want: true},
		{name: "heic unsupported", filePath: "folder/image.heic", want: false},
		{name: "png unsupported", filePath: "image.png", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFFmpegSupported(tt.filePath)
			if got != tt.want {
				t.Fatalf("unexpected support result for %q: got %v want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestFFmpegGenerator_UnsupportedFileExtension(t *testing.T) {
	generator := NewFFmpegThumbsGenerator(
		&fakeVideoFrameExtractor{},
		&fakeImageThumbsGenerator{},
	)

	err := generator.Generate(context.Background(), ThumbnailMeta{OrigFileRelPath: "clip.avi"})
	if err == nil {
		t.Fatalf("expected unsupported extension error")
	}
}
func TestFFmpegGenerator_ExtractsAndDelegatesToImageGenerator(t *testing.T) {
	tempDir := t.TempDir()

	frameExtractor := &fakeVideoFrameExtractor{}
	imageGenerator := &fakeImageThumbsGenerator{}
	generator := NewFFmpegThumbsGenerator(frameExtractor, imageGenerator)

	meta := ThumbnailMeta{
		OrigFilesRootDir: "/tmp/originals",
		OrigFileRelPath:  filepath.Join("videos", "clip.mov"),
		ThumbFileAbsDir:  tempDir,
		ThumbWidths:      []int{128, 256},
	}

	err := generator.Generate(context.Background(), meta)
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}

	if frameExtractor.callCount != 1 {
		t.Fatalf("expected frame extractor to be called once, got %d", frameExtractor.callCount)
	}

	expectedInput := filepath.Join(meta.OrigFilesRootDir, meta.OrigFileRelPath)
	if frameExtractor.calledFrom != expectedInput {
		t.Fatalf("unexpected extractor input path: got %q want %q", frameExtractor.calledFrom, expectedInput)
	}

	if imageGenerator.callCount != 1 {
		t.Fatalf("expected image generator to be called once, got %d", imageGenerator.callCount)
	}

	if imageGenerator.calledMeta.OrigFileRelPath != "clip.jpg" {
		t.Fatalf("unexpected delegated original filename: got %q want %q", imageGenerator.calledMeta.OrigFileRelPath, "clip.jpg")
	}

	if imageGenerator.calledMeta.ThumbFileAbsDir != meta.ThumbFileAbsDir {
		t.Fatalf("thumb output dir should be preserved: got %q want %q", imageGenerator.calledMeta.ThumbFileAbsDir, meta.ThumbFileAbsDir)
	}

	if imageGenerator.calledMeta.OrigFilesRootDir != meta.ThumbFileAbsDir {
		t.Fatalf("delegated orig root should be thumbs dir: got %q want %q", imageGenerator.calledMeta.OrigFilesRootDir, meta.ThumbFileAbsDir)
	}

	if _, err := os.Stat(frameExtractor.calledInto); !os.IsNotExist(err) {
		t.Fatalf("expected temporary frame file to be removed, stat error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "clip.jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected intermediary frame file to be removed, stat error: %v", err)
	}
}

func TestFFmpegGenerator_CleansTempWorkspaceOnDelegateFailure(t *testing.T) {
	tempDir := t.TempDir()

	frameExtractor := &fakeVideoFrameExtractor{}
	imageGenerator := &fakeImageThumbsGenerator{err: errors.New("boom")}
	generator := NewFFmpegThumbsGenerator(frameExtractor, imageGenerator)

	meta := ThumbnailMeta{
		OrigFilesRootDir: "/tmp/originals",
		OrigFileRelPath:  "clip.mp4",
		ThumbFileAbsDir:  tempDir,
	}

	err := generator.Generate(context.Background(), meta)
	if err == nil {
		t.Fatalf("expected delegate failure error")
	}

	if _, statErr := os.Stat(filepath.Join(tempDir, "clip.jpg")); !os.IsNotExist(statErr) {
		t.Fatalf("expected intermediary frame file to be removed on failure, stat error: %v", statErr)
	}
}
