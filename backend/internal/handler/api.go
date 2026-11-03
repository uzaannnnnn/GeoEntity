package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"

	"github.com/google/uuid"
)

type Store interface {
	List(context.Context) ([]entity.Entity, error)
	Get(context.Context, string) (entity.Entity, error)
	Create(context.Context, entity.Entity) error
	Update(context.Context, entity.Entity) error
	Delete(context.Context, string) error
}

type entityQueryStore interface {
	QueryEntities(context.Context, repository.EntityQuery) (repository.EntityPage, error)
	MapEntities(context.Context, repository.EntityFilters, repository.MapBounds) ([]repository.MapEntity, error)
	MapEntityExtent(context.Context, repository.EntityFilters) (repository.MapExtent, error)
}

type API struct {
	store  Store
	logger *slog.Logger
}

type adminAuditStore interface {
	ListAdminAudit(context.Context, int, string, string) ([]repository.AdminAuditEvent, error)
}

func New(store Store, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &API{store: store, logger: logger}
}

type errorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Field   string        `json:"field,omitempty"`
	Details []importIssue `json:"details,omitempty"`
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/admin/audit" {
		a.adminAudit(w, r)
		return
	}
	if r.URL.Path == "/api/geofences" || strings.HasPrefix(r.URL.Path, "/api/geofences/") {
		a.geofences(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/entities/") && strings.HasSuffix(r.URL.Path, "/geofences") {
		a.entityGeofences(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/exports/") || strings.HasPrefix(r.URL.Path, "/api/imports/") {
		a.exchange(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/attribute-definitions") || strings.HasPrefix(r.URL.Path, "/api/tags") || strings.HasPrefix(r.URL.Path, "/api/categories") || (strings.HasPrefix(r.URL.Path, "/api/entities/") && (strings.HasSuffix(r.URL.Path, "/attributes") || strings.HasSuffix(r.URL.Path, "/tags") || strings.HasSuffix(r.URL.Path, "/categories") || strings.HasSuffix(r.URL.Path, "/activity"))) {
		a.advanced(w, r)
		return
	}
	if r.URL.Path == "/api/health" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "ok"}})
		return
	}
	if r.URL.Path == "/api/entities/query" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.queryEntities(w, r)
		return
	}
	if r.URL.Path == "/api/entities/map/extent" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.mapEntityExtent(w, r)
		return
	}
	if r.URL.Path == "/api/entities/map" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.mapEntities(w, r)
		return
	}
	if r.URL.Path == "/api/entities" {
		switch r.Method {
		case http.MethodGet:
			a.list(w, r)
		case http.MethodPost:
			a.create(w, r)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/entities/") {
		id := strings.TrimPrefix(r.URL.Path, "/api/entities/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
			return
		}
		switch r.Method {
		case http.MethodGet:
			a.get(w, r, id)
		case http.MethodPut:
			a.update(w, r, id)
		case http.MethodDelete:
			a.delete(w, r, id)
		default:
			methodNotAllowed(w)
		}
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
}

func (a *API) adminAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	store, ok := a.store.(adminAuditStore)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	values := r.URL.Query()
	for key, all := range values {
		if key != "limit" && key != "cursor" || len(all) != 1 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid audit query parameters", "")
			return
		}
	}
	limit := 50
	if raw := values.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid audit query parameters", "limit")
			return
		}
		limit = n
	}
	beforeTime, beforeID := "", ""
	if cursor := values.Get("cursor"); cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid audit query parameters", "cursor")
			return
		}
		parts := strings.SplitN(string(decoded), "\x00", 2)
		if len(parts) != 2 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid audit query parameters", "cursor")
			return
		}
		if _, err = time.Parse(time.RFC3339Nano, parts[0]); err != nil || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid audit query parameters", "cursor")
			return
		}
		beforeTime, beforeID = parts[0], parts[1]
	}
	items, err := store.ListAdminAudit(r.Context(), limit+1, beforeTime, beforeID)
	if err != nil {
		a.internalError(w, err)
		return
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = base64.RawURLEncoding.EncodeToString([]byte(last.OccurredAt + "\x00" + last.ID))
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "next_cursor": next})
}

func (a *API) queryEntities(w http.ResponseWriter, r *http.Request) {
	values, ok := parseQueryValues(w, r, map[string]bool{"q": true, "type": true, "status": true, "limit": true, "cursor": true})
	if !ok {
		return
	}
	filters := repository.EntityFilters{Q: values.Get("q"), Type: values.Get("type"), Status: values.Get("status")}
	limit := 0
	if raw, exists := values["limit"]; exists {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", "limit")
			return
		}
		limit = parsed
	}
	store, ok := a.queryStore(w)
	if !ok {
		return
	}
	page, err := store.QueryEntities(r.Context(), repository.EntityQuery{EntityFilters: filters, Limit: limit, Cursor: values.Get("cursor")})
	if errors.Is(err, repository.ErrInvalidQuery) || errors.Is(err, repository.ErrInvalidCursor) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) mapEntities(w http.ResponseWriter, r *http.Request) {
	values, ok := parseQueryValues(w, r, map[string]bool{"q": true, "type": true, "status": true, "west": true, "south": true, "east": true, "north": true})
	if !ok {
		return
	}
	filters := repository.EntityFilters{Q: values.Get("q"), Type: values.Get("type"), Status: values.Get("status")}
	bounds, ok := parseMapBounds(w, values)
	if !ok {
		return
	}
	store, ok := a.queryStore(w)
	if !ok {
		return
	}
	items, err := store.MapEntities(r.Context(), filters, bounds)
	if errors.Is(err, repository.ErrInvalidQuery) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", "")
		return
	}
	if errors.Is(err, repository.ErrInvalidBounds) {
		writeError(w, http.StatusBadRequest, "INVALID_BOUNDS", "Map bounds are invalid", "bounds")
		return
	}
	if errors.Is(err, repository.ErrMapResultLimit) {
		writeError(w, http.StatusRequestEntityTooLarge, "MAP_RESULT_LIMIT", "More than 5000 entities match this map view; zoom in to see markers", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *API) mapEntityExtent(w http.ResponseWriter, r *http.Request) {
	values, ok := parseQueryValues(w, r, map[string]bool{"q": true, "type": true, "status": true})
	if !ok {
		return
	}
	filters := repository.EntityFilters{Q: values.Get("q"), Type: values.Get("type"), Status: values.Get("status")}
	store, ok := a.queryStore(w)
	if !ok {
		return
	}
	extent, err := store.MapEntityExtent(r.Context(), filters)
	if errors.Is(err, repository.ErrInvalidQuery) {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": extent})
}

func (a *API) queryStore(w http.ResponseWriter) (entityQueryStore, bool) {
	store, ok := a.store.(entityQueryStore)
	if !ok {
		a.internalError(w, errors.New("entity query store unavailable"))
		return nil, false
	}
	return store, true
}

func parseQueryValues(w http.ResponseWriter, r *http.Request, allowed map[string]bool) (url.Values, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", "")
		return nil, false
	}
	for key, items := range values {
		if !allowed[key] || len(items) != 1 {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "Invalid entity query parameters", key)
			return nil, false
		}
	}
	return values, true
}

func parseMapBounds(w http.ResponseWriter, values url.Values) (repository.MapBounds, bool) {
	read := func(key string, min, max float64) (float64, bool) {
		items, ok := values[key]
		if !ok || len(items) != 1 || items[0] == "" {
			writeError(w, http.StatusBadRequest, "INVALID_BOUNDS", "Map bounds are invalid", key)
			return 0, false
		}
		value, err := strconv.ParseFloat(items[0], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < min || value > max {
			writeError(w, http.StatusBadRequest, "INVALID_BOUNDS", "Map bounds are invalid", key)
			return 0, false
		}
		return value, true
	}
	west, ok := read("west", -180, 180)
	if !ok {
		return repository.MapBounds{}, false
	}
	south, ok := read("south", -90, 90)
	if !ok {
		return repository.MapBounds{}, false
	}
	east, ok := read("east", -180, 180)
	if !ok {
		return repository.MapBounds{}, false
	}
	north, ok := read("north", -90, 90)
	if !ok {
		return repository.MapBounds{}, false
	}
	if south > north {
		writeError(w, http.StatusBadRequest, "INVALID_BOUNDS", "Map bounds are invalid", "bounds")
		return repository.MapBounds{}, false
	}
	return repository.MapBounds{West: west, South: south, East: east, North: north}, true
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.List(r.Context())
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *API) get(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(w, id) {
		return
	}
	item, err := a.store.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	if err := input.NormalizeAndValidate(); err != nil {
		validationError(w, err)
		return
	}
	now := time.Now().UTC()
	item := entity.Entity{ID: uuid.NewString(), Name: input.Name, Type: input.Type, Description: input.Description, Status: input.Status, Latitude: input.Latitude, Longitude: input.Longitude, CreatedAt: now, UpdatedAt: now}
	if err := a.store.Create(r.Context(), item); err != nil {
		if errors.Is(err, repository.ErrActivityPayloadTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Activity snapshot exceeds the maximum size", "")
			return
		}
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (a *API) update(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(w, id) {
		return
	}
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	if err := input.NormalizeAndValidate(); err != nil {
		validationError(w, err)
		return
	}
	previous, err := a.store.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	item := entity.Entity{ID: previous.ID, Name: input.Name, Type: input.Type, Description: input.Description, Status: input.Status, Latitude: input.Latitude, Longitude: input.Longitude, CreatedAt: previous.CreatedAt, UpdatedAt: time.Now().UTC()}
	if err := a.store.Update(r.Context(), item); errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found", "")
		return
	} else if err != nil {
		if errors.Is(err, repository.ErrActivityPayloadTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Activity snapshot exceeds the maximum size", "")
			return
		}
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (a *API) delete(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(w, id) {
		return
	}
	if err := a.store.Delete(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found", "")
		return
	} else if err != nil {
		if errors.Is(err, repository.ErrActivityPayloadTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Activity snapshot exceeds the maximum size", "")
			return
		}
		a.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type inputJSON struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
}

func decodeInput(w http.ResponseWriter, r *http.Request) (entity.Input, bool) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Content-Type must be application/json", "")
		return entity.Input{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw inputJSON
	if err := decoder.Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Request body must contain valid JSON", "")
		return entity.Input{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Request body must contain exactly one JSON object", "")
		return entity.Input{}, false
	}
	if raw.Latitude == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Latitude is required", "latitude")
		return entity.Input{}, false
	}
	if raw.Longitude == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Longitude is required", "longitude")
		return entity.Input{}, false
	}
	return entity.Input{Name: raw.Name, Type: raw.Type, Description: raw.Description, Status: raw.Status, Latitude: *raw.Latitude, Longitude: *raw.Longitude}, true
}

func validID(w http.ResponseWriter, id string) bool {
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Entity ID must be a valid UUID", "id")
		return false
	}
	return true
}

func validationError(w http.ResponseWriter, err error) {
	field := ""
	switch {
	case errors.Is(err, entity.ErrNameRequired):
		field = "name"
	case errors.Is(err, entity.ErrInvalidType):
		field = "type"
	case errors.Is(err, entity.ErrInvalidStatus):
		field = "status"
	case errors.Is(err, entity.ErrInvalidLatitude):
		field = "latitude"
	case errors.Is(err, entity.ErrInvalidLongitude):
		field = "longitude"
	}
	writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), field)
}

func (a *API) internalError(w http.ResponseWriter, err error) {
	a.logger.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred", "")
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, PUT, DELETE")
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, map[string]any{"error": errorBody{Code: code, Message: message, Field: field}})
}

func writeImportError(w http.ResponseWriter, status int, code, message string, details []importIssue) {
	if len(details) > maxImportIssues {
		details = details[:maxImportIssues]
	}
	writeJSON(w, status, map[string]any{"error": errorBody{Code: code, Message: message, Details: details}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return
	}
}
