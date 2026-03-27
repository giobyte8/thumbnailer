package frameextractor

import "context"

type VideoFrameExtractor interface {
	Extract(ctx context.Context, fromAbsPath string, intoAbsPath string) error
}
