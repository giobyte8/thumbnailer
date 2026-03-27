package formatconverter

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

type HeifConvertFormatConverter struct {
	toolPath            string
	runner              commandRunner
	supportedInputExts  []string
	supportedOutputExts []string
}

func NewHeifConvertFormatConverter() *HeifConvertFormatConverter {
	return &HeifConvertFormatConverter{
		toolPath:            "heif-convert",
		runner:              &execCommandRunner{},
		supportedInputExts:  []string{".heic"},
		supportedOutputExts: []string{".jpg"},
	}
}

func (c *HeifConvertFormatConverter) HEICToJPEG(
	ctx context.Context,
	fromAbsPath string,
	intoAbsPath string,
) error {
	if err := c.validateInputExt(fromAbsPath); err != nil {
		return err
	}

	if err := c.validateOutputExt(intoAbsPath); err != nil {
		return err
	}

	args := c.mkHeifConvertArgs(fromAbsPath, intoAbsPath)
	output, err := c.runner.CombinedOutput(ctx, c.toolPath, args...)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("heif-convert binary not found: %w", err)
		}

		return fmt.Errorf(
			"heif-convert failed for %s: %w. output: %s",
			fromAbsPath,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

func (c *HeifConvertFormatConverter) mkHeifConvertArgs(
	inputFileAbsPath string,
	outputFileAbsPath string,
) []string {
	return []string{inputFileAbsPath, outputFileAbsPath}
}

func (c *HeifConvertFormatConverter) validateInputExt(filePath string) error {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if containsExtension(c.supportedInputExts, fileExtension) {
		return nil
	}

	return fmt.Errorf(
		"unsupported input extension for heif-convert format converter: %s",
		fileExtension,
	)
}

func (c *HeifConvertFormatConverter) validateOutputExt(filePath string) error {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if containsExtension(c.supportedOutputExts, fileExtension) {
		return nil
	}

	return fmt.Errorf(
		"unsupported output extension for heif-convert format converter: %s",
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
