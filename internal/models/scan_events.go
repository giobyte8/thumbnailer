package models

import (
	"github.com/google/uuid"
)

type FileDiscoveryEvent struct {
	ScanRequestID uuid.UUID `json:"scanRequestId"`
	EventType     string    `json:"eventType"`
	FilePath      string    `json:"filePath"`
}
