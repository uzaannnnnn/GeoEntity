package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
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

type API struct {
	store  Store
	logger *slog.Logger
}

func New(store Store, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &API{store: store, logger: logger}
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/health" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "ok"}})
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return
	}
}
