package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geoentity/backend/internal/geofence"
	"geoentity/backend/internal/repository"

	"github.com/google/uuid"
)

type geofenceStore interface {
	Geofences(context.Context, repository.GeofenceFilters, int, string) (repository.GeofencePage, error)
	GeofenceEvents(context.Context, string, repository.GeofenceEventFilters, int, string) (repository.GeofenceEventPage, error)
	EntityGeofences(context.Context, string) ([]repository.EntityGeofenceMembership, error)
	GetGeofence(context.Context, string) (geofence.Definition, error)
	CreateGeofence(context.Context, geofence.Definition) error
	UpdateGeofence(context.Context, geofence.Definition) error
	DeleteGeofence(context.Context, string) error
}

type geofenceInput struct {
	Name     string          `json:"name"`
	Enabled  *bool           `json:"enabled"`
	Geometry json.RawMessage `json:"geometry"`
}

func (a *API) geofences(w http.ResponseWriter, r *http.Request) {
	store, ok := a.geofenceStore(w)
	if !ok {
		return
	}
	if r.URL.Path == "/api/geofences" {
		switch r.Method {
		case http.MethodGet:
			a.listGeofences(w, r, store)
		case http.MethodPost:
			a.createGeofence(w, r, store)
		default:
			methodNotAllowed(w)
		}
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/geofences/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "events" {
		if !validGeofenceID(w, parts[0]) {
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.listGeofenceEvents(w, r, store, parts[0])
		return
	}
	id := path
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	if !validGeofenceID(w, id) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := store.GetGeofence(r.Context(), id)
		a.writeGeofenceResult(w, item, err, http.StatusOK)
	case http.MethodPut:
		a.updateGeofence(w, r, store, id)
	case http.MethodDelete:
		if err := store.DeleteGeofence(r.Context(), id); errors.Is(err, repository.ErrGeofenceNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Geofence not found", "")
		} else if err != nil {
			a.internalError(w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		methodNotAllowed(w)
	}
}

func (a *API) entityGeofences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := parseQueryValues(w, r, map[string]bool{}); !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if !strings.HasSuffix(path, "/geofences") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	id := strings.TrimSuffix(path, "/geofences")
	id = strings.TrimSuffix(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	if !validID(w, id) {
		return
	}
	store, ok := a.geofenceStore(w)
	if !ok {
		return
	}
	items, err := store.EntityGeofences(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *API) geofenceStore(w http.ResponseWriter) (geofenceStore, bool) {
	store, ok := a.store.(geofenceStore)
	if !ok {
		a.internalError(w, errors.New("geofence store unavailable"))
		return nil, false
	}
	return store, true
}

func (a *API) listGeofences(w http.ResponseWriter, r *http.Request, store geofenceStore) {
	values, ok := parseQueryValues(w, r, map[string]bool{"enabled": true, "limit": true, "cursor": true})
	if !ok {
		return
	}
	filters := repository.GeofenceFilters{}
	if raw, exists := values["enabled"]; exists {
		if raw[0] != "true" && raw[0] != "false" {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence query parameters", "enabled")
			return
		}
		enabled := raw[0] == "true"
		filters.Enabled = &enabled
	}
	limit := 0
	if raw, exists := values["limit"]; exists {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence query parameters", "limit")
			return
		}
		limit = parsed
	}
	page, err := store.Geofences(r.Context(), filters, limit, values.Get("cursor"))
	if errors.Is(err, repository.ErrInvalidQuery) || errors.Is(err, repository.ErrInvalidCursor) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence query parameters", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) listGeofenceEvents(w http.ResponseWriter, r *http.Request, store geofenceStore, geofenceID string) {
	values, ok := parseQueryValues(w, r, map[string]bool{"entity_id": true, "from": true, "to": true, "limit": true, "cursor": true})
	if !ok {
		return
	}
	filters := repository.GeofenceEventFilters{EntityID: values.Get("entity_id")}
	if filters.EntityID != "" {
		if _, err := uuid.Parse(filters.EntityID); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence event query parameters", "entity_id")
			return
		}
	}
	if raw, exists := values["from"]; exists {
		parsed, err := time.Parse(time.RFC3339Nano, raw[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Event time filters must be RFC3339 timestamps", "from")
			return
		}
		utc := parsed.UTC()
		filters.From = &utc
	}
	if raw, exists := values["to"]; exists {
		parsed, err := time.Parse(time.RFC3339Nano, raw[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Event time filters must be RFC3339 timestamps", "to")
			return
		}
		utc := parsed.UTC()
		filters.To = &utc
	}
	if filters.From != nil && filters.To != nil && filters.From.After(*filters.To) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "The from time must not be after the to time", "from")
		return
	}
	limit := 0
	if raw, exists := values["limit"]; exists {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence event query parameters", "limit")
			return
		}
		limit = parsed
	}
	page, err := store.GeofenceEvents(r.Context(), geofenceID, filters, limit, values.Get("cursor"))
	if errors.Is(err, repository.ErrGeofenceNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Geofence not found", "")
		return
	}
	if errors.Is(err, repository.ErrInvalidQuery) || errors.Is(err, repository.ErrInvalidCursor) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid geofence event query parameters", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) createGeofence(w http.ResponseWriter, r *http.Request, store geofenceStore) {
	input, ok := decodeGeofenceInput(w, r)
	if !ok {
		return
	}
	item, field, err := makeGeofence(input)
	if err != nil {
		geofenceValidationError(w, field, err)
		return
	}
	now := time.Now().UTC()
	item.ID = uuid.NewString()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := store.CreateGeofence(r.Context(), item); errors.Is(err, repository.ErrGeofenceNameConflict) {
		writeError(w, http.StatusConflict, "CONFLICT", "A geofence with this name already exists", "name")
		return
	} else if errors.Is(err, repository.ErrGeofenceLimit) {
		writeError(w, http.StatusConflict, "GEOFENCE_LIMIT", "Maximum active geofence count reached", "enabled")
		return
	} else if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (a *API) updateGeofence(w http.ResponseWriter, r *http.Request, store geofenceStore, id string) {
	previous, err := store.GetGeofence(r.Context(), id)
	if errors.Is(err, repository.ErrGeofenceNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Geofence not found", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	input, ok := decodeGeofenceInput(w, r)
	if !ok {
		return
	}
	item, field, err := makeGeofence(input)
	if err != nil {
		geofenceValidationError(w, field, err)
		return
	}
	item.ID = previous.ID
	item.CreatedAt = previous.CreatedAt
	item.UpdatedAt = time.Now().UTC()
	if err := store.UpdateGeofence(r.Context(), item); errors.Is(err, repository.ErrGeofenceNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Geofence not found", "")
		return
	} else if errors.Is(err, repository.ErrGeofenceNameConflict) {
		writeError(w, http.StatusConflict, "CONFLICT", "A geofence with this name already exists", "name")
		return
	} else if errors.Is(err, repository.ErrGeofenceLimit) {
		writeError(w, http.StatusConflict, "GEOFENCE_LIMIT", "Maximum active geofence count reached", "enabled")
		return
	} else if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func makeGeofence(input geofenceInput) (geofence.Definition, string, error) {
	name, _, err := geofence.NormalizeName(input.Name)
	if err != nil {
		return geofence.Definition{}, "name", err
	}
	if input.Enabled == nil {
		return geofence.Definition{}, "enabled", errors.New("enabled is required")
	}
	geometry, err := geofence.ParseGeometry(input.Geometry)
	if err != nil {
		return geofence.Definition{}, "geometry", err
	}
	return geofence.Definition{Name: name, Enabled: *input.Enabled, Geometry: geometry}, "", nil
}

func decodeGeofenceInput(w http.ResponseWriter, r *http.Request) (geofenceInput, bool) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Content-Type must be application/json", "")
		return geofenceInput{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var input geofenceInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body exceeds the maximum size", "")
			return geofenceInput{}, false
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Request body must contain valid JSON", "")
		return geofenceInput{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Request body must contain exactly one JSON object", "")
		return geofenceInput{}, false
	}
	if len(bytes.TrimSpace(input.Geometry)) == 0 || bytes.Equal(bytes.TrimSpace(input.Geometry), []byte("null")) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Geometry is required", "geometry")
		return geofenceInput{}, false
	}
	return input, true
}

func geofenceValidationError(w http.ResponseWriter, field string, err error) {
	code := "VALIDATION_ERROR"
	writeError(w, http.StatusBadRequest, code, err.Error(), field)
}

func validGeofenceID(w http.ResponseWriter, id string) bool {
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Geofence ID must be a valid UUID", "id")
		return false
	}
	return true
}

func (a *API) writeGeofenceResult(w http.ResponseWriter, item geofence.Definition, err error, status int) {
	if errors.Is(err, repository.ErrGeofenceNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Geofence not found", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"data": item})
}
