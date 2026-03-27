package formatconverter

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

func TestHeifConvertFormatConverter_MkHeifConvertArgs(t *testing.T) {
	converter := NewHeifConvertFormatConverter()
	got := converter.mkHeifConvertArgs("/tmp/in.heic", "/tmp/out.jpg")
	want := []string{"/tmp/in.heic", "/tmp/out.jpg"}

	if len(got) != len(want) {
		t.Fatalf("unexpected args length: got %d want %d", len(got), len(want))
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected arg at index %d: got %q want %q", index, got[index], want[index])
		}
	}
}

func TestHeifConvertFormatConverter_UnsupportedInputExt(t *testing.T) {
	runner := &fakeCommandRunner{}
	converter := NewHeifConvertFormatConverter()
	converter.runner = runner

	err := converter.HEICToJPEG(context.Background(), "/tmp/in.png", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected unsupported input extension error")
	}

	if !strings.Contains(err.Error(), "unsupported input extension") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if runner.calls != 0 {
		t.Fatalf("runner should not be called for unsupported input extension")
	}
}

func TestHeifConvertFormatConverter_UnsupportedOutputExt(t *testing.T) {
	runner := &fakeCommandRunner{}
	converter := NewHeifConvertFormatConverter()
	converter.runner = runner

	err := converter.HEICToJPEG(context.Background(), "/tmp/in.heic", "/tmp/out.png")
	if err == nil {
		t.Fatalf("expected unsupported output extension error")
	}

	if !strings.Contains(err.Error(), "unsupported output extension") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if runner.calls != 0 {
		t.Fatalf("runner should not be called for unsupported output extension")
	}
}

func TestHeifConvertFormatConverter_BinaryNotFound(t *testing.T) {
	runner := &fakeCommandRunner{
		err: &exec.Error{Name: "heif-convert", Err: exec.ErrNotFound},
	}
	converter := NewHeifConvertFormatConverter()
	converter.runner = runner

	err := converter.HEICToJPEG(context.Background(), "/tmp/in.heic", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected binary-not-found error")
	}

	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected wrapped exec.ErrNotFound, got: %v", err)
	}

	if !strings.Contains(err.Error(), "binary not found") {
		t.Fatalf("expected binary not found message, got: %v", err)
	}
}

func TestHeifConvertFormatConverter_CommandFailureIncludesOutput(t *testing.T) {
	runner := &fakeCommandRunner{
		output: []byte("conversion failed"),
		err:    errors.New("exit status 1"),
	}
	converter := NewHeifConvertFormatConverter()
	converter.runner = runner

	err := converter.HEICToJPEG(context.Background(), "/tmp/in.heic", "/tmp/out.jpg")
	if err == nil {
		t.Fatalf("expected command failure")
	}

	if !strings.Contains(err.Error(), "output: conversion failed") {
		t.Fatalf("expected converter output in error, got: %v", err)
	}
}

func TestHeifConvertFormatConverter_Integration(t *testing.T) {
	if _, err := exec.LookPath("heif-convert"); err != nil {
		t.Skip("heif-convert not available, skipping integration test")
	}

	inputPath := fixturePath(t, "sample.heic")
	outputPath := filepath.Join(t.TempDir(), "converted.jpg")

	converter := NewHeifConvertFormatConverter()
	err := converter.HEICToJPEG(context.Background(), inputPath, outputPath)
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
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
		t.Fatalf("failed to decode converted image: %v", err)
	}

	if config.Width <= 0 || config.Height <= 0 {
		t.Fatalf("invalid converted image dimensions: %dx%d", config.Width, config.Height)
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
