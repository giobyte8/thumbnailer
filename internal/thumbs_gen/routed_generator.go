package thumbsgen

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

type RoutedThumbsGenerator struct {
	routes map[string]ThumbsGenerator
}

func NewRoutedThumbsGenerator(routes map[string]ThumbsGenerator) *RoutedThumbsGenerator {
	normalized := make(map[string]ThumbsGenerator, len(routes))
	for extension, generator := range routes {
		normalized[strings.ToLower(extension)] = generator
	}

	return &RoutedThumbsGenerator{routes: normalized}
}

func NewDefaultRoutedThumbsGenerator(
	ffmpegGenerator ThumbsGenerator,
	lilliputGenerator ThumbsGenerator,
) *RoutedThumbsGenerator {
	return NewRoutedThumbsGenerator(map[string]ThumbsGenerator{
		".mp4":  ffmpegGenerator,
		".mov":  ffmpegGenerator,
		".jpg":  lilliputGenerator,
		".jpeg": lilliputGenerator,
		".png":  lilliputGenerator,
		".heic": lilliputGenerator,
	})
}

func (g *RoutedThumbsGenerator) Generate(
	ctx context.Context,
	meta ThumbnailMeta,
) error {
	extension := strings.ToLower(filepath.Ext(meta.OrigFileRelPath))
	generator, found := g.routes[extension]
	if !found {
		slog.Warn(
			"Discarding unsupported file extension for thumbnail generation",
			"filePath",
			meta.OrigFileRelPath,
			"extension",
			extension,
		)
		return nil
	}

	if generator == nil {
		return fmt.Errorf(
			"thumbnail generator route for extension %s is not configured",
			extension,
		)
	}

	return generator.Generate(ctx, meta)
}
