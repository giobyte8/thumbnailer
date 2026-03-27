package frameextractor

import (
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	output []byte
	err    error
	calls  int
}

func (r *fakeCommandRunner) CombinedOutput(
	_ context.Context,
	_ string,
	_ ...string,
) ([]byte, error) {
	r.calls++
	return r.output, r.err
}

func TestFFmpegFrameExtractor_MkFFmpegArgs(t *testing.T) {
	extractor := NewFFmpegFrameExtractor()
	got := extractor.mkFFmpegArgs("/tmp/in.mov", "/tmp/out.jpg")
	want := []string{
		"-y",
		"-ss", "00:00:01",
		"-i", "/tmp/in.mov",
		"-vframes", "1",
		"-vf", "format=yuv420p",
		"-q:v", "2",
		"/tmp/out.jpg",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected args length: got %d want %d", len(got), len(want))
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected ffmpeg arg at index %d: got %q want %q", index, got[index], want[index])
		}
	}
}

func TestFFmpegFrameExtractor_UnsupportedInputExt(t *testing.T) {
	runner := &fakeCommandRunner{}
	extractor := NewFFmpegFrameExtractor()
	extractor.runner = runner

	err := extractor.Extract(context.Background(), "/tmp/input.avi", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected error for unsupported input extension")
	}

	if !strings.Contains(err.Error(), "unsupported input extension") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if runner.calls != 0 {
		t.Fatalf("runner should not be called for invalid input extension")
	}
}

func TestFFmpegFrameExtractor_UnsupportedOutputExt(t *testing.T) {
	runner := &fakeCommandRunner{}
	extractor := NewFFmpegFrameExtractor()
	extractor.runner = runner

	err := extractor.Extract(context.Background(), "/tmp/input.mov", "/tmp/out.png")
	if err == nil {
		t.Fatalf("expected error for unsupported output extension")
	}

	if !strings.Contains(err.Error(), "unsupported output extension") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if runner.calls != 0 {
		t.Fatalf("runner should not be called for invalid output extension")
	}
}

func TestFFmpegFrameExtractor_BinaryNotFound(t *testing.T) {
	runner := &fakeCommandRunner{
		err: &exec.Error{Name: "ffmpeg", Err: exec.ErrNotFound},
	}
	extractor := NewFFmpegFrameExtractor()
	extractor.runner = runner

	err := extractor.Extract(context.Background(), "/tmp/input.mov", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected error when ffmpeg binary is missing")
	}

	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected wrapped exec.ErrNotFound, got: %v", err)
	}

	if !strings.Contains(err.Error(), "binary not found") {
		t.Fatalf("expected binary not found message, got: %v", err)
	}
}

func TestFFmpegFrameExtractor_CommandFailureIncludesOutput(t *testing.T) {
	runner := &fakeCommandRunner{
		output: []byte("boom\nreason"),
		err:    errors.New("exit status 1"),
	}
	extractor := NewFFmpegFrameExtractor()
	extractor.runner = runner

	err := extractor.Extract(context.Background(), "/tmp/input.mov", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected command failure error")
	}

	if !strings.Contains(err.Error(), "output: boom\nreason") {
		t.Fatalf("expected command output in error, got: %v", err)
	}
}

func TestFFmpegFrameExtractor_Integration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	inputPath := fixturePath(t, "sample.mov")
	outputPath := filepath.Join(t.TempDir(), "frame.jpg")

	extractor := NewFFmpegFrameExtractor()
	err := extractor.Extract(context.Background(), inputPath, outputPath)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	if fileInfo.Size() <= 0 {
		t.Fatalf("expected non-empty output file")
	}

	fileHandle, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("failed to open output image: %v", err)
	}
	defer fileHandle.Close()

	config, _, err := image.DecodeConfig(fileHandle)
	if err != nil {
		t.Fatalf("failed to decode output image: %v", err)
	}

	if config.Width <= 0 || config.Height <= 0 {
		t.Fatalf("invalid extracted image dimensions: %dx%d", config.Width, config.Height)
	}
}

func fixturePath(t *testing.T, fileName string) string {
	t.Helper()

	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve current file path")
	}

	path := filepath.Join(filepath.Dir(currentFilePath), "testdata", fileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture not found at %s: %v", path, err)
	}

	return path
}
