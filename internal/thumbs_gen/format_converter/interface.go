package formatconverter

import "context"

type ImageFormatConverter interface {
	HEICToJPEG(ctx context.Context, fromAbsPath string, intoAbsPath string) error
}
