package models

import (
	"github.com/google/uuid"
)

// DeviceMetadataInstance represents a key-value pair of device metadata in the database
type DeviceMetadataInstance struct {
	Base
	DeviceID uuid.UUID `json:"device_id"`
	Key      string    `json:"key"`
	Value    string    `json:"value"`
}

// DeviceMetadata represents all device metadata in the API
type DeviceMetadata struct {
	DeviceID uuid.UUID         `json:"device_id"`
	Metadata map[string]string `json:"metadata"`
}

// DeviceMetadataValue represents a device metadata value for a specific key in the API
type DeviceMetadataValue struct {
	Value string `json:"value"`
}
