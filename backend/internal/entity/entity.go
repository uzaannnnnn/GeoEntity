package entity

import (
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrNameRequired     = errors.New("name is required")
	ErrInvalidType      = errors.New("type must be one of: vehicle, iot_device, facility, other")
	ErrInvalidStatus    = errors.New("status must be one of: active, inactive, maintenance")
	ErrInvalidLatitude  = errors.New("latitude must be a finite number between -90 and 90")
	ErrInvalidLongitude = errors.New("longitude must be a finite number between -180 and 180")
)

var allowedTypes = map[string]struct{}{
	"vehicle": {}, "iot_device": {}, "facility": {}, "other": {},
}

var allowedStatuses = map[string]struct{}{
	"active": {}, "inactive": {}, "maintenance": {},
}

type Entity struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Input struct {
	Name        string
	Type        string
	Description string
	Status      string
	Latitude    float64
	Longitude   float64
}

func (in *Input) NormalizeAndValidate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return ErrNameRequired
	}
	if _, ok := allowedTypes[in.Type]; !ok {
		return ErrInvalidType
	}
	if _, ok := allowedStatuses[in.Status]; !ok {
		return ErrInvalidStatus
	}
	if math.IsNaN(in.Latitude) || math.IsInf(in.Latitude, 0) || in.Latitude < -90 || in.Latitude > 90 {
		return ErrInvalidLatitude
	}
	if math.IsNaN(in.Longitude) || math.IsInf(in.Longitude, 0) || in.Longitude < -180 || in.Longitude > 180 {
		return ErrInvalidLongitude
	}
	return nil
}
