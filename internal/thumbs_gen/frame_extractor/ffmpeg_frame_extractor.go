package frameextractor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type commandRunner interface {
	CombinedOutput(ctx context.Context, commandPath string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (r *execCommandRunner) CombinedOutput(
	ctx context.Context,
	commandPath string,
	args ...string,
) ([]byte, error) {
	command := exec.CommandContext(ctx, commandPath, args...)
	return command.CombinedOutput()
}

type FFmpegFrameExtractor struct {
	ffmpegPath          string
	runner              commandRunner
	supportedInputExts  []string
	supportedOutputExts []string
}

func NewFFmpegFrameExtractor() *FFmpegFrameExtractor {
	return &FFmpegFrameExtractor{
		ffmpegPath:          "ffmpeg",
		runner:              &execCommandRunner{},
		supportedInputExts:  []string{".mp4", ".mov"},
		supportedOutputExts: []string{".jpg"},
	}
}

func (e *FFmpegFrameExtractor) Extract(
	ctx context.Context,
	fromAbsPath string,
	intoAbsPath string,
) error {
	if err := e.validateInputExt(fromAbsPath); err != nil {
		return err
	}

	if err := e.validateOutputExt(intoAbsPath); err != nil {
		return err
	}

	args := e.mkFFmpegArgs(fromAbsPath, intoAbsPath)
	output, err := e.runner.CombinedOutput(ctx, e.ffmpegPath, args...)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("ffmpeg binary not found: %w", err)
		}

		return fmt.Errorf(
			"ffmpeg frame extraction failed for %s: %w. output: %s",
			fromAbsPath,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

func (e *FFmpegFrameExtractor) mkFFmpegArgs(
	inputFileAbsPath string,
	outputFileAbsPath string,
) []string {
	return []string{
		"-y",
		"-ss", "00:00:01",
		"-i", inputFileAbsPath,
		"-vframes", "1",
		"-vf", "format=yuv420p",
		"-q:v", "2",
		outputFileAbsPath,
	}
}

func (e *FFmpegFrameExtractor) validateInputExt(filePath string) error {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if containsExtension(e.supportedInputExts, fileExtension) {
		return nil
	}

	return fmt.Errorf(
		"unsupported input extension for ffmpeg frame extractor: %s",
		fileExtension,
	)
}

func (e *FFmpegFrameExtractor) validateOutputExt(filePath string) error {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if containsExtension(e.supportedOutputExts, fileExtension) {
		return nil
	}

	return fmt.Errorf(
		"unsupported output extension for ffmpeg frame extractor: %s",
		fileExtension,
	)
}

func containsExtension(supportedExtensions []string, extension string) bool {
	for _, supportedExtension := range supportedExtensions {
		if supportedExtension == extension {
			return true
		}
	}

	return false
}
