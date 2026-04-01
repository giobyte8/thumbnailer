package format

type Format string

const (
	JPEG Format = "jpeg"
	PNG  Format = "png"
	WEBP Format = "webp"
	HEIF Format = "heif"

	MOV Format = "mov"
	MP4 Format = "mp4"
	M4V Format = "m4v"

	UNSUPPORTED Format = "unsupported"
)
