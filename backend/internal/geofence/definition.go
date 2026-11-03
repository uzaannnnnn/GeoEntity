package geofence

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidName = errors.New("name must contain 1 to 100 non-whitespace characters")

const MaxActiveGeofences = 100

type Definition struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Geometry  Geometry  `json:"geometry"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NormalizeName(name string) (display, normalized string, err error) {
	display = strings.TrimSpace(name)
	if !utf8.ValidString(display) || utf8.RuneCountInString(display) < 1 || utf8.RuneCountInString(display) > 100 {
		return "", "", ErrInvalidName
	}
	normalized = strings.ToLower(display)
	if utf8.RuneCountInString(normalized) > 100 {
		return "", "", ErrInvalidName
	}
	return display, normalized, nil
}
