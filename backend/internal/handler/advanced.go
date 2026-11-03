package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"geoentity/backend/internal/repository"
)

type advancedStore interface {
	Definitions(context.Context, string) ([]repository.Definition, error)
	Definition(context.Context, string) (repository.Definition, error)
	SaveDefinition(context.Context, repository.Definition, bool) error
	AttributeValues(context.Context, string) (string, map[string]any, map[string]any, error)
	ReplaceAttributes(context.Context, string, map[string]any) error
	Catalog(context.Context, string) ([]repository.CatalogItem, error)
	CreateCatalog(context.Context, string, string) (repository.CatalogItem, error)
	DeleteCatalog(context.Context, string, string) error
	EntityCatalog(context.Context, string, string) ([]repository.CatalogItem, error)
	ReplaceCatalog(context.Context, string, string, []string) error
	Activities(context.Context, string, int, string) (repository.ActivityPage, error)
}

func (a *API) advanced(w http.ResponseWriter, r *http.Request) {
	s, ok := a.store.(advancedStore)
	if !ok {
		a.internalError(w, errors.New("advanced store unavailable"))
		return
	}
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[0] == "api" && parts[1] == "attribute-definitions" {
		a.definitions(w, r, s)
		return
	}
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "attribute-definitions" {
		a.definitions(w, r, s)
		return
	}
	if len(parts) == 2 && parts[0] == "api" && (parts[1] == "tags" || parts[1] == "categories") {
		a.catalog(w, r, s, parts[1])
		return
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "entities" {
		id, section := parts[2], parts[3]
		if !validID(w, id) {
			return
		}
		switch section {
		case "attributes":
			a.attributes(w, r, s, id)
		case "tags", "categories":
			a.entityCatalog(w, r, s, id, section)
		case "activity":
			a.activity(w, r, s, id)
		default:
			writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
		}
		return
	}
	if len(parts) == 3 && parts[0] == "api" && (parts[1] == "tags" || parts[1] == "categories") {
		if r.Method != http.MethodDelete {
			methodNotAllowed(w)
			return
		}
		if !validID(w, parts[2]) {
			return
		}
		e := s.DeleteCatalog(r.Context(), parts[1], parts[2])
		switch {
		case errors.Is(e, repository.ErrNotFound):
			writeError(w, 404, "NOT_FOUND", "Catalog item not found", "")
		case e != nil:
			if strings.Contains(e.Error(), "FOREIGN KEY") {
				writeError(w, 409, "CONFLICT", "Catalog item is assigned to entities", "")
			} else {
				a.internalError(w, e)
			}
		default:
			w.WriteHeader(204)
		}
		return
	}
	writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, 400, "INVALID_JSON", "Content-Type must be application/json", "")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		var maxErr *http.MaxBytesError
		if errors.As(e, &maxErr) {
			writeError(w, 413, "REQUEST_TOO_LARGE", "Request body exceeds the maximum size", "")
			return false
		}
		writeError(w, 400, "INVALID_JSON", "Request body must contain valid JSON", "")
		return false
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		writeError(w, 400, "INVALID_JSON", "Request body must contain exactly one JSON object", "")
		return false
	}
	return true
}
func (a *API) definitions(w http.ResponseWriter, r *http.Request, s advancedStore) {
	switch r.Method {
	case http.MethodGet:
		typ := r.URL.Query().Get("entity_type")
		if !validEntityType(typ) {
			writeError(w, 400, "VALIDATION_ERROR", "entity_type is required and must be supported", "entity_type")
			return
		}
		v, e := s.Definitions(r.Context(), typ)
		if e != nil {
			a.internalError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"data": v})
	case http.MethodPost:
		var d repository.Definition
		if !decodeStrict(w, r, &d) {
			return
		}
		if e := validateDefinition(d); e != nil {
			writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
			return
		}
		e := s.SaveDefinition(r.Context(), d, true)
		if e != nil {
			if strings.Contains(e.Error(), "UNIQUE") || strings.Contains(e.Error(), "maximum") {
				writeError(w, 409, "CONFLICT", e.Error(), "")
			} else {
				a.internalError(w, e)
			}
			return
		}
		v, _ := s.Definitions(r.Context(), d.EntityType)
		for _, x := range v {
			if x.Key == d.Key {
				writeJSON(w, 201, map[string]any{"data": x})
				return
			}
		}
	case http.MethodPut:
		prefix := "/api/attribute-definitions/"
		id := strings.TrimPrefix(r.URL.Path, prefix)
		if id == r.URL.Path || strings.Contains(id, "/") {
			writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
			return
		}
		if !validID(w, id) {
			return
		}
		var body struct {
			Label     string   `json:"label"`
			Required  bool     `json:"required"`
			MinNumber *float64 `json:"min_number"`
			MaxNumber *float64 `json:"max_number"`
			MaxLength *int     `json:"max_length"`
			Archived  bool     `json:"archived"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		d, e := s.Definition(r.Context(), id)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Definition not found", "")
			return
		}
		if e != nil {
			a.internalError(w, e)
			return
		}
		d.Label = body.Label
		d.Required = body.Required
		d.MinNumber = body.MinNumber
		d.MaxNumber = body.MaxNumber
		d.MaxLength = body.MaxLength
		d.Archived = body.Archived
		if e := validateDefinition(d); e != nil {
			writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
			return
		}
		e = s.SaveDefinition(r.Context(), d, false)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Definition not found", "")
		} else if e != nil {
			if strings.Contains(e.Error(), "immutable") || strings.Contains(e.Error(), "invalidate") {
				writeError(w, 409, "CONFLICT", e.Error(), "")
			} else {
				writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
			}
			return
		}
		v, _ := s.Definitions(r.Context(), d.EntityType)
		for _, x := range v {
			if x.ID == id {
				writeJSON(w, 200, map[string]any{"data": x})
				return
			}
		}
	default:
		methodNotAllowed(w)
	}
}

var attributeKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var reserved = map[string]bool{"id": true, "name": true, "type": true, "description": true, "status": true, "latitude": true, "longitude": true, "created_at": true, "updated_at": true, "tags": true, "categories": true, "attributes": true}

func validEntityType(t string) bool {
	return t == "vehicle" || t == "iot_device" || t == "facility" || t == "other"
}
func validateDefinition(d repository.Definition) error {
	if !validEntityType(d.EntityType) || !attributeKey.MatchString(d.Key) || reserved[d.Key] || strings.TrimSpace(d.Label) == "" || len([]rune(d.Label)) > 100 {
		return errors.New("invalid entity_type, key, or label")
	}
	if d.ValueType != "string" && d.ValueType != "number" && d.ValueType != "boolean" {
		return errors.New("value_type must be string, number, or boolean")
	}
	if d.ValueType != "number" && (d.MinNumber != nil || d.MaxNumber != nil) {
		return errors.New("numeric bounds require number type")
	}
	if d.ValueType != "string" && d.MaxLength != nil {
		return errors.New("max_length requires string type")
	}
	if d.MaxLength != nil && (*d.MaxLength < 1 || *d.MaxLength > 2048) {
		return errors.New("max_length must be between 1 and 2048")
	}
	if d.MinNumber != nil && d.MaxNumber != nil && *d.MinNumber > *d.MaxNumber {
		return errors.New("min_number exceeds max_number")
	}
	if d.MinNumber != nil && (math.IsNaN(*d.MinNumber) || math.IsInf(*d.MinNumber, 0)) || d.MaxNumber != nil && (math.IsNaN(*d.MaxNumber) || math.IsInf(*d.MaxNumber, 0)) {
		return errors.New("numeric bounds must be finite")
	}
	return nil
}
func (a *API) attributes(w http.ResponseWriter, r *http.Request, s advancedStore, id string) {
	switch r.Method {
	case http.MethodGet:
		typ, values, previous, e := s.AttributeValues(r.Context(), id)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Entity not found", "")
			return
		}
		if e != nil {
			a.internalError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"data": map[string]any{"entity_type": typ, "values": values, "previous_type_values": previous}})
	case http.MethodPut:
		var body struct {
			Values map[string]any `json:"values"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		if body.Values == nil {
			writeError(w, 400, "VALIDATION_ERROR", "values must be an object", "values")
			return
		}
		e := s.ReplaceAttributes(r.Context(), id, body.Values)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Entity not found", "")
		} else if errors.Is(e, repository.ErrActivityPayloadTooLarge) {
			writeError(w, 413, "REQUEST_TOO_LARGE", "Activity snapshot exceeds the maximum size", "")
		} else if e != nil {
			writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
		} else {
			writeJSON(w, 200, map[string]any{"data": map[string]any{"values": body.Values}})
		}
	default:
		methodNotAllowed(w)
	}
}
func (a *API) catalog(w http.ResponseWriter, r *http.Request, s advancedStore, kind string) {
	switch r.Method {
	case http.MethodGet:
		v, e := s.Catalog(r.Context(), kind)
		if e != nil {
			a.internalError(w, e)
		} else {
			writeJSON(w, 200, map[string]any{"data": v})
		}
	case http.MethodPost:
		var b struct {
			Name string `json:"name"`
		}
		if !decodeStrict(w, r, &b) {
			return
		}
		v, e := s.CreateCatalog(r.Context(), kind, b.Name)
		if e != nil {
			if strings.Contains(e.Error(), "UNIQUE") {
				writeError(w, 409, "CONFLICT", "A catalog item with that name already exists", "")
			} else {
				writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
			}
			return
		}
		writeJSON(w, 201, map[string]any{"data": v})
	default:
		methodNotAllowed(w)
	}
}
func (a *API) entityCatalog(w http.ResponseWriter, r *http.Request, s advancedStore, id, kind string) {
	switch r.Method {
	case http.MethodGet:
		v, e := s.EntityCatalog(r.Context(), id, kind)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Entity not found", "")
		} else if e != nil {
			a.internalError(w, e)
		} else {
			writeJSON(w, 200, map[string]any{"data": v})
		}
	case http.MethodPut:
		var ids []string
		if kind == "tags" {
			var b struct {
				TagIDs *[]string `json:"tag_ids"`
			}
			if !decodeStrict(w, r, &b) {
				return
			}
			if b.TagIDs == nil || *b.TagIDs == nil {
				writeError(w, 400, "VALIDATION_ERROR", "tag_ids must be an array", "tag_ids")
				return
			}
			ids = *b.TagIDs
		} else {
			var b struct {
				CategoryIDs *[]string `json:"category_ids"`
			}
			if !decodeStrict(w, r, &b) {
				return
			}
			if b.CategoryIDs == nil || *b.CategoryIDs == nil {
				writeError(w, 400, "VALIDATION_ERROR", "category_ids must be an array", "category_ids")
				return
			}
			ids = *b.CategoryIDs
		}
		e := s.ReplaceCatalog(r.Context(), id, kind, ids)
		if errors.Is(e, repository.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "Entity not found", "")
		} else if e != nil {
			writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
		} else {
			v, e := s.EntityCatalog(r.Context(), id, kind)
			if e != nil {
				a.internalError(w, e)
			} else {
				writeJSON(w, 200, map[string]any{"data": v})
			}
		}
	default:
		methodNotAllowed(w)
	}
}
func (a *API) activity(w http.ResponseWriter, r *http.Request, s advancedStore, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 0
	if q := r.URL.Query().Get("limit"); q != "" {
		n, e := strconv.Atoi(q)
		if e != nil || n == 0 {
			writeError(w, 400, "VALIDATION_ERROR", "limit must be an integer", "limit")
			return
		}
		limit = n
	}
	p, e := s.Activities(r.Context(), id, limit, r.URL.Query().Get("cursor"))
	if e != nil {
		writeError(w, 400, "VALIDATION_ERROR", e.Error(), "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": p.Data, "meta": map[string]any{"next_cursor": p.NextCursor}})
}
