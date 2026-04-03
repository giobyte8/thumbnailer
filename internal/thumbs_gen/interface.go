package thumbsgen

import (
	"context"

	"github.com/giobyte8/thumbnailer/internal/format"
)

const ThumbsExtension = ".webp"
const ThumbsQuality = 60

// ThumbnailMeta holds all the necessary metadata for generating
// thumbnails for a specific original image file.
type ThumbnailMeta struct {

	// Absolute path to the root directory where all original files
	// are stored. Notice this is the 'root' directory, files might
	// live in subdirectories inside it.
	OrigFilesRootDir string

	// Path to the original file, relative to the OrigFilesRootDir.
	OrigFileRelPath string

	// Absolute path to the directory where thumbnail file should be
	// stored
	ThumbFileAbsDir string

	// List of width sizes in pixels of thumbnails to generate.
	// For example, if this is [100, 200, 300], then three thumbnails
	// will be generated with widths 100px, 200px, and 300px,
	ThumbWidths []int
}

type ThumbsGenerator interface {
	// Generate generates thumbnails for the original file specified in meta.
	//
	// This method checks if the format of the original file is supported
	// before performing the generation. Use GenerateWithoutFormatsCheck if
	// you need to skip such checks.
	Generate(ctx context.Context, meta ThumbnailMeta) error

	// GenerateWithoutFormatsCheck generates thumbnails without checking
	// whether the format of the original file is supported.
	//
	// Useful for high-throughput scenarios where the caller has already
	// validated the format, avoiding extra reads of file header bytes.
	GenerateWithoutFormatsCheck(
		ctx context.Context,
		meta ThumbnailMeta,
		origFileFormat format.Format,
	) error
}

type ImgDimensions struct {
	Width  int
	Height int
}
