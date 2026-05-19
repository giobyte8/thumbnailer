package services

import (
	"context"
	"fmt"          // Required for image.Config and image.DecodeConfig
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/giobyte8/thumbnailer/internal/models"
	thumbsgen "github.com/giobyte8/thumbnailer/internal/thumbs_gen"
)

type ThumbnailsConfig struct {
	DirOriginalsRoot  string
	DirThumbnailsRoot string
	ThumbnailWidths   []int
}

type ThumbnailsService struct {
	config         ThumbnailsConfig
	thumbGenerator thumbsgen.ThumbsGenerator
}

func NewThumbnailsService(
	config ThumbnailsConfig,
	thumbGenerator thumbsgen.ThumbsGenerator,
) *ThumbnailsService {
	return &ThumbnailsService{
		config:         config,
		thumbGenerator: thumbGenerator,
	}
}

func (s *ThumbnailsService) ProcessGenRequest(
	ctx context.Context,
	req models.ThumbRequest,
) error {
	slog.Debug(
		"Processing thumbnail generation request",
		"filePath",
		req.FilePath,
	)

	err := s.cleanupExisting(ctx, req.FilePath)
	if err != nil {
		return err
	}

	thumbMeta, err := s.prepareThumbnailMeta(req.FilePath)
	if err != nil {
		return err
	}

	err = s.thumbGenerator.Generate(ctx, *thumbMeta)
	if err != nil {
		return err
	}

	return nil
}

func (s *ThumbnailsService) ProcessDelRequest(
	ctx context.Context,
	req models.ThumbRequest,
) error {
	slog.Debug(
		"Processing thumbnail delete request",
		"filePath",
		req.FilePath,
	)

	return s.cleanupExisting(ctx, req.FilePath)
}

func (s *ThumbnailsService) cleanupExisting(
	ctx context.Context,
	origFileRelPath string,
) error {

	// Determine sub directory for thumbnails
	origFileRelDir := filepath.Dir(origFileRelPath)
	thumbsDir := filepath.Join(s.config.DirThumbnailsRoot, origFileRelDir)
	if _, err := os.Stat(thumbsDir); os.IsNotExist(err) {
		return nil
	}

	// Prepare wildcard patterns to match existing thumbnails with
	// width suffixes of exactly 3 or 4 digits (e.g. 320px, 1080px).
	baseName := filepath.Base(origFileRelPath)
	ext := filepath.Ext(baseName)
	fileNameNoExt := strings.TrimSuffix(baseName, ext)
	pattern3Digits := filepath.Join(
		thumbsDir,
		fmt.Sprintf(
			"%s_[0-9][0-9][0-9]px%s",
			fileNameNoExt,
			thumbsgen.ThumbsExtension,
		),
	)
	pattern4Digits := filepath.Join(
		thumbsDir,
		fmt.Sprintf(
			"%s_[0-9][0-9][0-9][0-9]px%s",
			fileNameNoExt,
			thumbsgen.ThumbsExtension,
		),
	)

	// Find files matching either pattern
	matches3Digits, err := filepath.Glob(pattern3Digits)
	if err != nil {
		return fmt.Errorf(
			"failed to glob for existing thumbnails with pattern %s: %w",
			pattern3Digits,
			err,
		)
	}
	matches4Digits, err := filepath.Glob(pattern4Digits)
	if err != nil {
		return fmt.Errorf(
			"failed to glob for existing thumbnails with pattern %s: %w",
			pattern4Digits,
			err,
		)
	}

	matches := append(matches3Digits, matches4Digits...)

	// Remove each file mathing pattern
	for _, matchPath := range matches {
		select {
		case <-ctx.Done():
			slog.Warn(
				"Context cancelled during thumbnail cleanup.",
				"path",
				matchPath,
			)
			return ctx.Err()
		default:
			// Continue with deletion
		}

		slog.Debug("Removing existing thumbnail", "path", matchPath)
		if err := os.Remove(matchPath); err != nil {
			return fmt.Errorf(
				"failed to remove existing thumbnail %s: %w",
				matchPath,
				err,
			)
		}

		// TODO Remove direcotory if empty after removing thumbnails
	}

	return nil
}

func (s *ThumbnailsService) prepareThumbnailMeta(
	origFileRelPath string,
) (*thumbsgen.ThumbnailMeta, error) {
	thumbMeta := new(thumbsgen.ThumbnailMeta)
	thumbMeta.OrigFilesRootDir = s.config.DirOriginalsRoot
	thumbMeta.OrigFileRelPath = origFileRelPath

	// Determine output directory for thumbnails
	origFileRelDir := filepath.Dir(origFileRelPath)
	thumbMeta.ThumbFileAbsDir = filepath.Join(
		s.config.DirThumbnailsRoot,
		origFileRelDir,
	)

	// TODO: Consider move dir creation to the moment of thumbnail saving
	if _, err := os.Stat(thumbMeta.ThumbFileAbsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(thumbMeta.ThumbFileAbsDir, 0755); err != nil {
			return nil, fmt.Errorf(
				"failed to create thumbnails directory %s: %w",
				thumbMeta.ThumbFileAbsDir,
				err,
			)
		}
	}

	thumbMeta.ThumbWidths = s.config.ThumbnailWidths
	return thumbMeta, nil
}
