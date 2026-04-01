package format

import (
	"fmt"
	"io"
	"os"

	"github.com/h2non/filetype"
)

type FormatDetector struct{}

func NewFormatDetector() *FormatDetector {
	return &FormatDetector{}
}

func (d *FormatDetector) Detect(absFilePath string) (Format, error) {

	format, err := detectWithFileType(absFilePath)
	if err != nil {
		return UNSUPPORTED, fmt.Errorf(
			"failed to detect format: %w",
			err)
	}

	return format, nil
}

func detectWithFileType(absFilePath string) (Format, error) {

	// filetype needs at least 261 bytes
	header, err := firstNBytes(absFilePath, 261)
	if err != nil {
		return UNSUPPORTED, fmt.Errorf(
			"failed to read file header for format detection: %w",
			err)
	}

	kind, err := filetype.Match(header)
	if err != nil {
		return UNSUPPORTED, fmt.Errorf(
			"failed to detect format with filetype: %w",
			err)
	}

	switch kind.MIME.Value {
	case "image/jpeg":
		return JPEG, nil
	case "image/png":
		return PNG, nil
	case "image/webp":
		return WEBP, nil
	case "image/heif":
		return HEIF, nil

	case "video/quicktime":
		return MOV, nil
	case "video/mp4":
		return MP4, nil
	case "video/x-m4v":
		return M4V, nil

	// For other formats, we use UNSUPPORTED to delegate detection to next
	// detector in the chain (e.g. vips).
	default:
		return UNSUPPORTED, nil
	}
}

func firstNBytes(absFilePath string, nBytes int) ([]byte, error) {

	// Open file for reading (File is not loaded into memory)
	file, err := os.Open(absFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read just the first n bytes to detect format
	header := make([]byte, nBytes)
	readBytesCount, err := file.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// Use only the bytes actually read
	header = header[:readBytesCount]

	return header, nil
}
