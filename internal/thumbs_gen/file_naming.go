package thumbsgen

import (
	"fmt"
	"path/filepath"
	"strings"
)

func baseNameNoExt(meta ThumbnailMeta) string {
	fileBaseName := filepath.Base(meta.OrigFileRelPath)

	return strings.TrimSuffix(
		fileBaseName,
		filepath.Ext(fileBaseName),
	)
}

func mkOriginalFileAbsPath(meta ThumbnailMeta) string {
	return filepath.Join(meta.OrigFilesRootDir, meta.OrigFileRelPath)
}

func mkThumbFileAbsPath(
	meta ThumbnailMeta,
	thumbWidth int,
	thumbExtension string,
) string {
	thumbFileName := fmt.Sprintf(
		"%s_%dpx%s",
		baseNameNoExt(meta),
		thumbWidth,
		thumbExtension,
	)

	return filepath.Join(meta.ThumbFileAbsDir, thumbFileName)
}

// mkIntermediaryThumbFileAbsPath creates an absolute path for a thumbnail file
// that matches name from original file BUT has the given extension (e.g. .jpg).
//
// This is useful for intermediary thumbnail files that are not final
// thumbnails but we still want them to have a name that is easily identifiable
// and traceable back to the original file.
// Some examples are:
//   - extracted frame from video (lake_hdr.mov -> lake_hdr.jpg)
//   - HEIC converted to JPEG before final thumbnails (sample.heic -> sample.jpg)
func mkIntermediaryThumbFileAbsPath(meta ThumbnailMeta, extension string) string {
	return filepath.Join(
		meta.ThumbFileAbsDir,
		fmt.Sprintf("%s%s", baseNameNoExt(meta), extension),
	)
}
