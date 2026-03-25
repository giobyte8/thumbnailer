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