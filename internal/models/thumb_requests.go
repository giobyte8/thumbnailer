package models

import (
	"github.com/google/uuid"
)

type ThumbRequest struct {
	ThumbRequestId uuid.UUID `json:"thumbRequestId"`

	// Path to original media file, relative to env
	// variable 'DIR_ORIGINALS_ROOT'
	FilePath string `json:"filePath"`
}
