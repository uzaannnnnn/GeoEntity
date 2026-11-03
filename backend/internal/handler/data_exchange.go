package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"

	"github.com/google/uuid"
)

const (
	maxImportBytes      = 5 << 20
	maxImportRows       = 5000
	maxImportField      = 1 << 20
	maxImportAttributes = 32 << 10
	maxImportMetadata   = 64 << 10
	maxImportIssues     = 100
	maxExportRows       = 100000
)

var csvHeader = []string{"id", "name", "type", "description", "status", "latitude", "longitude", "created_at", "updated_at", "attributes_json", "previous_type_values_json", "tags_json", "categories_json"}
var requiredCSVHeader = map[string]bool{"name": true, "type": true, "status": true, "latitude": true, "longitude": true}
var errImportLimit = errors.New("import limit exceeded")

type exchangeStore interface {
	CountExportEntities(context.Context, repository.EntityFilters, int) (int, error)
	ExportEntityBatch(context.Context, repository.EntityFilters, string, string, int) ([]repository.ExportEntity, error)
	PreflightImport(context.Context, []repository.ImportRecord) (repository.ImportPreflight, error)
	CreateImportedEntities(context.Context, string, string, []repository.ImportRecord) (int, int, error)
}

type importIssue struct {
	Row     int    `json:"row,omitempty"`
	Feature int    `json:"feature,omitempty"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type importWarning struct {
	Row     int    `json:"row,omitempty"`
	Feature int    `json:"feature,omitempty"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type importPreview struct {
	Format                       string              `json:"format"`
	TotalRows                    int                 `json:"total_rows"`
	ValidRows                    int                 `json:"valid_rows"`
	InvalidRows                  int                 `json:"invalid_rows"`
	Warnings                     []importWarning     `json:"warnings"`
	Errors                       []importIssue       `json:"errors"`
	ErrorsTruncated              bool                `json:"errors_truncated"`
	WarningsTruncated            bool                `json:"warnings_truncated"`
	DuplicateCandidatesTruncated bool                `json:"duplicate_candidates_truncated"`
	ExistingIDsTruncated         bool                `json:"existing_ids_truncated"`
	NewCatalogItemsTruncated     bool                `json:"new_catalog_items_truncated"`
	DuplicateCandidates          []map[string]any    `json:"duplicate_candidates"`
	ExistingIDs                  []string            `json:"existing_ids"`
	NewCatalogItems              []map[string]string `json:"new_catalog_items"`
	PreviewSHA256                string              `json:"preview_sha256"`
	IDMode                       string              `json:"id_mode"`
	IDMap                        []map[string]string `json:"id_map"`
}
type importResult struct {
	BatchID           string              `json:"batch_id"`
	Format            string              `json:"format"`
	Imported          int                 `json:"imported"`
	Created           int                 `json:"created"`
	TagsCreated       int                 `json:"tags_created"`
	CategoriesCreated int                 `json:"categories_created"`
	IDMap             []map[string]string `json:"id_map"`
}
type parsedImport struct {
	Record   repository.ImportRecord
	Index    int
	Warnings []importWarning
	Issues   []importIssue
}

func (a *API) exchange(w http.ResponseWriter, r *http.Request) {
	store, ok := a.store.(exchangeStore)
	if !ok {
		a.internalError(w, errors.New("data exchange store unavailable"))
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "exports" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.export(w, r, store, parts[2])
		return
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "imports" && (parts[2] == "csv" || parts[2] == "geojson") && (parts[3] == "preview" || parts[3] == "commit") {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		a.importRequest(w, r, store, parts[2], parts[3])
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", "")
}

func (a *API) export(w http.ResponseWriter, r *http.Request, store exchangeStore, format string) {
	if format != "entities.csv" && format != "entities.geojson" {
		writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	allowed := map[string]bool{"q": true, "type": true, "status": true}
	if format == "entities.csv" {
		allowed["spreadsheet_safe"] = true
	}
	values, ok := parseQueryValues(w, r, allowed)
	if !ok {
		return
	}
	filters := repository.EntityFilters{Q: values.Get("q"), Type: values.Get("type"), Status: values.Get("status")}
	count, err := store.CountExportEntities(r.Context(), filters, maxExportRows)
	if errors.Is(err, repository.ErrInvalidQuery) {
		writeError(w, 400, "INVALID_QUERY", "Invalid entity query parameters", "")
		return
	}
	if errors.Is(err, repository.ErrExportResultLimit) {
		writeError(w, 413, "EXPORT_RESULT_LIMIT", "Export exceeds the 100000 entity limit", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if format == "entities.csv" {
		spreadsheetSafe := values.Get("spreadsheet_safe") == "true"
		if values.Get("spreadsheet_safe") != "" && values.Get("spreadsheet_safe") != "true" && values.Get("spreadsheet_safe") != "false" {
			writeError(w, 400, "INVALID_QUERY", "spreadsheet_safe must be true or false", "spreadsheet_safe")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="geoentity-entities-%s.csv"`, stamp))
		writer := csv.NewWriter(w)
		writer.UseCRLF = true
		if err := writer.Write(csvHeader); err != nil {
			return
		}
		if err := writeExportBatches(r.Context(), store, filters, func(item repository.ExportEntity) error { return writer.Write(csvRecord(item, spreadsheetSafe)) }); err != nil {
			return
		}
		writer.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="geoentity-entities-%s.geojson"`, stamp))
	_, _ = io.WriteString(w, `{"type":"FeatureCollection","features":[`)
	first := true
	err = writeExportBatches(r.Context(), store, filters, func(item repository.ExportEntity) error {
		feature := map[string]any{"type": "Feature", "id": item.ID, "geometry": map[string]any{"type": "Point", "coordinates": []float64{item.Longitude, item.Latitude}}, "properties": map[string]any{"name": item.Name, "type": item.Type, "description": item.Description, "status": item.Status, "created_at": item.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": item.UpdatedAt.UTC().Format(time.RFC3339Nano), "attributes": item.Attributes, "previous_type_values": item.PreviousTypeValues, "tags": item.Tags, "categories": item.Categories}}
		encoded, e := json.Marshal(feature)
		if e != nil {
			return e
		}
		if !first {
			if _, e = io.WriteString(w, ","); e != nil {
				return e
			}
		}
		first = false
		_, e = w.Write(encoded)
		return e
	})
	if err != nil {
		return
	}
	_, _ = io.WriteString(w, `]}`)
	_ = count
}

func writeExportBatches(ctx context.Context, store exchangeStore, filters repository.EntityFilters, write func(repository.ExportEntity) error) error {
	lastCreated, lastID := "", ""
	for {
		items, err := store.ExportEntityBatch(ctx, filters, lastCreated, lastID, 25)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for _, item := range items {
			if err := write(item); err != nil {
				return err
			}
		}
		last := items[len(items)-1]
		lastCreated = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		lastID = last.ID
		if len(items) < 25 {
			return nil
		}
	}
}
func csvRecord(item repository.ExportEntity, safe bool) []string {
	attrs, _ := json.Marshal(item.Attributes)
	previous, _ := json.Marshal(item.PreviousTypeValues)
	tags, _ := json.Marshal(item.Tags)
	categories, _ := json.Marshal(item.Categories)
	text := func(v string) string {
		if safe && formulaLike(v) {
			return "'" + v
		}
		return v
	}
	return []string{item.ID, text(item.Name), text(item.Type), text(item.Description), text(item.Status), strconv.FormatFloat(item.Latitude, 'f', -1, 64), strconv.FormatFloat(item.Longitude, 'f', -1, 64), item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano), string(attrs), string(previous), string(tags), string(categories)}
}
func formulaLike(value string) bool {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return true
	}
	return false
}

func (a *API) importRequest(w http.ResponseWriter, r *http.Request, store exchangeStore, format, action string) {
	values, ok := parseQueryValues(w, r, map[string]bool{"id_mode": true})
	if !ok {
		return
	}
	idMode := values.Get("id_mode")
	if idMode == "" {
		idMode = "preserve"
	}
	if idMode != "preserve" && idMode != "regenerate" {
		writeError(w, 400, "INVALID_IMPORT", "id_mode must be preserve or regenerate", "id_mode")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (format == "csv" && media != "text/csv") || (format == "geojson" && media != "application/geo+json" && media != "application/json") {
		writeError(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Unsupported import content type", "")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImportBytes))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, 413, "REQUEST_TOO_LARGE", "Import body exceeds 5 MiB", "")
		} else {
			writeError(w, 400, "INVALID_IMPORT", "Unable to read import body", "")
		}
		return
	}
	if len(body) == 0 {
		writeError(w, 400, "INVALID_IMPORT", "Import body is empty", "")
		return
	}
	fingerprint := importFingerprint(format, idMode, body)
	if action == "commit" && r.Header.Get("X-Preview-SHA256") != fingerprint {
		writeError(w, 409, "PREVIEW_STALE", "Import content or options differ from the preview", "")
		return
	}
	parsed, parseErr := parseImportBody(format, idMode, fingerprint, body)
	if parseErr != nil {
		if errors.Is(parseErr, errImportLimit) {
			writeError(w, 413, "IMPORT_LIMIT_EXCEEDED", "Import exceeds a supported record, field, or nesting limit", "")
			return
		}
		writeError(w, 400, "INVALID_IMPORT", parseErr.Error(), "")
		return
	}
	if len(parsed) > maxImportRows {
		writeError(w, 413, "IMPORT_LIMIT_EXCEEDED", "Import may contain at most 5000 records", "")
		return
	}
	if action == "preview" {
		a.previewImport(w, r, store, format, idMode, fingerprint, parsed)
		return
	}
	a.commitImport(w, r, store, format, idMode, fingerprint, parsed)
}

func importFingerprint(format, idMode string, body []byte) string {
	data := append([]byte(format+"\x00"+idMode+"\x00"), body...)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func parseImportBody(format, idMode, fingerprint string, body []byte) ([]parsedImport, error) {
	if !utf8.Valid(body) {
		return nil, errors.New("Import must be valid UTF-8")
	}
	if format == "csv" {
		return parseCSVImport(idMode, fingerprint, body)
	}
	return parseGeoJSONImport(idMode, fingerprint, body)
}

func parseCSVImport(idMode, fingerprint string, body []byte) ([]parsedImport, error) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false
	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("CSV header is missing or malformed")
	}
	if len(header) == 0 {
		return nil, errors.New("CSV header is empty")
	}
	columns := map[string]int{}
	for index, name := range header {
		name = strings.TrimPrefix(name, "\ufeff")
		if name == "" {
			return nil, errors.New("CSV contains an empty header")
		}
		if _, ok := columns[name]; ok {
			return nil, fmt.Errorf("duplicate CSV header %q", name)
		}
		columns[name] = index
		if !contains(csvHeader, name) {
			return nil, fmt.Errorf("unknown CSV header %q", name)
		}
	}
	for required := range requiredCSVHeader {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("required CSV column %q is missing", required)
		}
	}
	parsed := []parsedImport{}
	rowNumber := 1
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNumber++
		if err != nil {
			return nil, fmt.Errorf("CSV row %d is malformed", rowNumber)
		}
		if rowNumber > maxImportRows+1 {
			return nil, fmt.Errorf("%w: CSV exceeds 5000 data rows", errImportLimit)
		}
		values := map[string]string{}
		for name, index := range columns {
			if index < len(record) {
				values[name] = record[index]
			}
		}
		for _, name := range header {
			name = strings.TrimPrefix(name, "\ufeff")
			value := values[name]
			if len(value) > maxImportField {
				return nil, fmt.Errorf("%w: CSV row %d field %s exceeds 1 MiB", errImportLimit, rowNumber, name)
			}
		}
		item, warnings, issues := csvImportRecord(values, rowNumber, idMode, fingerprint)
		item.SourceIndex = rowNumber
		parsed = append(parsed, parsedImport{Record: item, Index: rowNumber, Warnings: warnings, Issues: issues})
	}
	if len(parsed) == 0 {
		return nil, errors.New("CSV contains a header but no data rows")
	}
	return parsed, nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func csvImportRecord(values map[string]string, row int, idMode, fingerprint string) (repository.ImportRecord, []importWarning, []importIssue) {
	warnings := []importWarning{}
	issues := []importIssue{}
	get := func(key string) string { return values[key] }
	var record repository.ImportRecord
	record.SourceID = get("id")
	id := record.SourceID
	if idMode == "regenerate" || id == "" {
		id = stableImportID(fingerprint, row)
	} else if uuid.Validate(id) != nil {
		issues = append(issues, importIssue{Row: row, Field: "id", Code: "INVALID_ID", Message: "ID must be a UUID"})
	}
	latitude, latErr := strconv.ParseFloat(get("latitude"), 64)
	longitude, lonErr := strconv.ParseFloat(get("longitude"), 64)
	input := entity.Input{Name: get("name"), Type: get("type"), Description: get("description"), Status: get("status"), Latitude: latitude, Longitude: longitude}
	if get("latitude") == "" || latErr != nil {
		issues = append(issues, importIssue{Row: row, Field: "latitude", Code: "INVALID_COORDINATE", Message: "Latitude must be a finite number between -90 and 90"})
	}
	if get("longitude") == "" || lonErr != nil {
		issues = append(issues, importIssue{Row: row, Field: "longitude", Code: "INVALID_COORDINATE", Message: "Longitude must be a finite number between -180 and 180"})
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		issues = append(issues, importIssue{Row: row, Field: "name", Code: "VALIDATION_ERROR", Message: entity.ErrNameRequired.Error()})
	}
	if input.Type != "vehicle" && input.Type != "iot_device" && input.Type != "facility" && input.Type != "other" {
		issues = append(issues, importIssue{Row: row, Field: "type", Code: "VALIDATION_ERROR", Message: entity.ErrInvalidType.Error()})
	}
	if input.Status != "active" && input.Status != "inactive" && input.Status != "maintenance" {
		issues = append(issues, importIssue{Row: row, Field: "status", Code: "VALIDATION_ERROR", Message: entity.ErrInvalidStatus.Error()})
	}
	if get("latitude") != "" && latErr == nil && (math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90) {
		issues = append(issues, importIssue{Row: row, Field: "latitude", Code: "INVALID_COORDINATE", Message: entity.ErrInvalidLatitude.Error()})
	}
	if get("longitude") != "" && lonErr == nil && (math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180) {
		issues = append(issues, importIssue{Row: row, Field: "longitude", Code: "INVALID_COORDINATE", Message: entity.ErrInvalidLongitude.Error()})
	}
	now := time.Now().UTC()
	created, updated := now, now
	if get("created_at") != "" || get("updated_at") != "" {
		if get("created_at") == "" || get("updated_at") == "" {
			issues = append(issues, importIssue{Row: row, Field: "created_at", Code: "INVALID_TIMESTAMP", Message: "created_at and updated_at must both be supplied"})
		} else {
			var err error
			created, err = time.Parse(time.RFC3339, get("created_at"))
			if err != nil {
				issues = append(issues, importIssue{Row: row, Field: "created_at", Code: "INVALID_TIMESTAMP", Message: "Timestamp must be RFC3339"})
			}
			updated, err = time.Parse(time.RFC3339, get("updated_at"))
			if err != nil {
				issues = append(issues, importIssue{Row: row, Field: "updated_at", Code: "INVALID_TIMESTAMP", Message: "Timestamp must be RFC3339"})
			}
		}
	}
	attrs, err := parseJSONMap(get("attributes_json"), maxImportAttributes)
	if err != nil {
		issues = append(issues, importIssue{Row: row, Field: "attributes_json", Code: "INVALID_METADATA_JSON", Message: "Expected a JSON object within 32 KiB"})
	}
	previous, err2 := parseJSONMap(get("previous_type_values_json"), maxImportMetadata)
	if err2 != nil {
		issues = append(issues, importIssue{Row: row, Field: "previous_type_values_json", Code: "INVALID_METADATA_JSON", Message: "Expected a JSON object"})
	}
	tags, tagErr := parseJSONStringArray(get("tags_json"), maxImportMetadata)
	if tagErr != nil {
		issues = append(issues, importIssue{Row: row, Field: "tags_json", Code: "INVALID_METADATA_JSON", Message: "Expected a JSON array of strings"})
	}
	categories, categoryErr := parseJSONStringArray(get("categories_json"), maxImportMetadata)
	if categoryErr != nil {
		issues = append(issues, importIssue{Row: row, Field: "categories_json", Code: "INVALID_METADATA_JSON", Message: "Expected a JSON array of strings"})
	}
	if len(get("previous_type_values_json"))+len(get("tags_json"))+len(get("categories_json")) > maxImportMetadata {
		issues = append(issues, importIssue{Row: row, Field: "metadata", Code: "METADATA_TOO_LARGE", Message: "Previous values, tags, and categories exceed 64 KiB combined"})
	}
	if idMode == "regenerate" && record.SourceID != "" {
		warnings = append(warnings, importWarning{Row: row, Field: "id", Code: "ID_REGENERATED", Message: "Source ID will be replaced"})
	}
	record.Entity = entity.Entity{ID: id, Name: input.Name, Type: input.Type, Description: input.Description, Status: input.Status, Latitude: input.Latitude, Longitude: input.Longitude, CreatedAt: created, UpdatedAt: updated}
	record.Attributes = attrs
	record.PreviousTypeValues = previous
	record.Tags = tags
	record.Categories = categories
	warnings = append(warnings, duplicateCatalogWarnings(tags, "tags_json", row, 0)...)
	warnings = append(warnings, duplicateCatalogWarnings(categories, "categories_json", row, 0)...)
	record.SourceIndex = row
	return record, warnings, issues
}

func parseJSONMap(raw string, max int) (map[string]any, error) {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	if len(raw) > max {
		return nil, errors.New("too large")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if depthExceeded(value, 0) {
		return nil, errors.New("JSON nesting exceeds maximum depth 16")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	converted, err := normalizeJSONNumbers(object)
	if err != nil {
		return nil, err
	}
	return converted.(map[string]any), nil
}
func parseJSONStringArray(raw string, max int) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	if len(raw) > max {
		return nil, errors.New("too large")
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil || items == nil {
		return nil, errors.New("not string array")
	}
	return items, nil
}
func normalizeJSONNumbers(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, errors.New("invalid number")
		}
		return n, nil
	case []any:
		for i := range v {
			x, err := normalizeJSONNumbers(v[i])
			if err != nil {
				return nil, err
			}
			v[i] = x
		}
		return v, nil
	case map[string]any:
		for k, item := range v {
			x, err := normalizeJSONNumbers(item)
			if err != nil {
				return nil, err
			}
			v[k] = x
		}
		return v, nil
	default:
		return value, nil
	}
}

func stableImportID(fingerprint string, index int) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("geoentity-import:%s:%d", fingerprint, index))).String()
}

func parseGeoJSONImport(idMode, fingerprint string, body []byte) ([]parsedImport, error) {
	if err := rejectDuplicateJSONKeys(body); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("GeoJSON must contain valid JSON")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("GeoJSON must contain one JSON value")
	}
	if depthExceeded(value, 0) {
		return nil, fmt.Errorf("%w: GeoJSON nesting exceeds maximum depth 16", errImportLimit)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("GeoJSON root must be Feature or FeatureCollection")
	}
	kind, ok := root["type"].(string)
	if !ok {
		return nil, errors.New("GeoJSON type is required")
	}
	for _, key := range sortedJSONMapKeys(root) {
		valid := key == "type" || key == "bbox" || (kind == "FeatureCollection" && key == "features") || (kind == "Feature" && (key == "id" || key == "geometry" || key == "properties"))
		if !valid {
			return nil, fmt.Errorf("unknown GeoJSON member %q", key)
		}
	}
	if bbox, ok := root["bbox"]; ok && !validGeoJSONBBox(bbox) {
		return nil, errors.New("GeoJSON bbox is invalid")
	}
	features := []any{}
	if kind == "FeatureCollection" {
		raw, ok := root["features"].([]any)
		if !ok {
			return nil, errors.New("FeatureCollection features must be an array")
		}
		features = raw
	} else if kind == "Feature" {
		features = append(features, root)
	} else {
		return nil, errors.New("only FeatureCollection or Feature is supported")
	}
	if len(features) == 0 {
		return nil, errors.New("GeoJSON contains no features")
	}
	if len(features) > maxImportRows {
		return nil, fmt.Errorf("%w: GeoJSON exceeds 5000 features", errImportLimit)
	}
	parsed := make([]parsedImport, 0, len(features))
	for i, raw := range features {
		feature, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("feature %d must be an object", i+1)
		}
		item, warnings, issues := geoFeatureRecord(feature, i+1, idMode, fingerprint)
		item.SourceIndex = i + 1
		parsed = append(parsed, parsedImport{Record: item, Index: i + 1, Warnings: warnings, Issues: issues})
	}
	return parsed, nil
}
func depthExceeded(value any, depth int) bool {
	if depth > 16 {
		return true
	}
	switch v := value.(type) {
	case []any:
		for _, x := range v {
			if depthExceeded(x, depth+1) {
				return true
			}
		}
	case map[string]any:
		for _, x := range v {
			if depthExceeded(x, depth+1) {
				return true
			}
		}
	}
	return false
}

func validGeoJSONBBox(value any) bool {
	coords, ok := value.([]any)
	if !ok || (len(coords) != 4 && len(coords) != 6) {
		return false
	}
	nums := make([]float64, len(coords))
	for i, item := range coords {
		n, ok := item.(json.Number)
		if !ok {
			return false
		}
		v, err := n.Float64()
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
		nums[i] = v
	}
	if nums[0] < -180 || nums[0] > 180 || nums[1] < -90 || nums[1] > 90 {
		return false
	}
	upper := len(nums) / 2
	if nums[upper] < -180 || nums[upper] > 180 || nums[upper+1] < -90 || nums[upper+1] > 90 {
		return false
	}
	return nums[1] <= nums[upper+1]
}

func rejectDuplicateJSONKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return errors.New("GeoJSON must contain valid JSON")
	}
	if err := scanJSONValue(decoder, token, 0); err != nil {
		if errors.Is(err, errImportLimit) {
			return err
		}
		return errors.New("GeoJSON contains duplicate object keys or invalid structure")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("GeoJSON must contain one JSON value")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, token json.Token, depth int) error {
	if depth > 16 {
		return fmt.Errorf("%w: JSON nesting exceeds maximum depth 16", errImportLimit)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return errors.New("duplicate object key")
			}
			seen[key] = true
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := scanJSONValue(decoder, value, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unclosed object")
		}
	case '[':
		for decoder.More() {
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := scanJSONValue(decoder, value, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unclosed array")
		}
	default:
		return errors.New("unexpected delimiter")
	}
	return nil
}
func geoFeatureRecord(feature map[string]any, index int, idMode, fingerprint string) (repository.ImportRecord, []importWarning, []importIssue) {
	warnings := []importWarning{}
	issues := []importIssue{}
	for _, k := range sortedJSONMapKeys(feature) {
		if k != "type" && k != "id" && k != "geometry" && k != "properties" && k != "bbox" {
			issues = append(issues, importIssue{Feature: index, Field: k, Code: "UNKNOWN_MEMBER", Message: "Unknown Feature member"})
		}
	}
	if feature["type"] != "Feature" {
		issues = append(issues, importIssue{Feature: index, Field: "type", Code: "INVALID_FEATURE", Message: "type must be Feature"})
	}
	props, ok := feature["properties"].(map[string]any)
	if !ok {
		issues = append(issues, importIssue{Feature: index, Field: "properties", Code: "INVALID_PROPERTIES", Message: "properties must be an object"})
		props = map[string]any{}
	}
	allowed := map[string]bool{"name": true, "type": true, "description": true, "status": true, "created_at": true, "updated_at": true, "attributes": true, "previous_type_values": true, "tags": true, "categories": true}
	for _, k := range sortedJSONMapKeys(props) {
		if !allowed[k] {
			issues = append(issues, importIssue{Feature: index, Field: "properties." + k, Code: "UNKNOWN_PROPERTY", Message: "Unknown property"})
		}
	}
	geometry, ok := feature["geometry"].(map[string]any)
	if !ok {
		issues = append(issues, importIssue{Feature: index, Field: "geometry", Code: "INVALID_GEOMETRY", Message: "geometry must be a Point"})
		geometry = map[string]any{}
	}
	for _, k := range sortedJSONMapKeys(geometry) {
		if k != "type" && k != "coordinates" && k != "bbox" {
			issues = append(issues, importIssue{Feature: index, Field: "geometry." + k, Code: "UNKNOWN_MEMBER", Message: "Unknown geometry member"})
		}
	}
	if geometry["type"] != "Point" {
		issues = append(issues, importIssue{Feature: index, Field: "geometry.type", Code: "UNSUPPORTED_GEOMETRY", Message: "Only Point geometry is supported"})
	}
	coordinates, ok := geometry["coordinates"].([]any)
	if !ok || len(coordinates) < 2 || len(coordinates) > 3 {
		issues = append(issues, importIssue{Feature: index, Field: "geometry.coordinates", Code: "INVALID_COORDINATE", Message: "Point coordinates must contain longitude, latitude, and optional altitude"})
		coordinates = []any{json.Number("0"), json.Number("0")}
	}
	parseCoord := func(pos int, field string, min, max float64) float64 {
		if pos >= len(coordinates) {
			return 0
		}
		n, ok := coordinates[pos].(json.Number)
		if !ok {
			issues = append(issues, importIssue{Feature: index, Field: field, Code: "INVALID_COORDINATE", Message: "Coordinate must be numeric"})
			return 0
		}
		v, err := n.Float64()
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
			issues = append(issues, importIssue{Feature: index, Field: field, Code: "INVALID_COORDINATE", Message: "Coordinate is outside WGS84 bounds"})
			return 0
		}
		return v
	}
	lon := parseCoord(0, "geometry.coordinates[0]", -180, 180)
	lat := parseCoord(1, "geometry.coordinates[1]", -90, 90)
	if len(coordinates) == 3 {
		alt, ok := coordinates[2].(json.Number)
		if !ok {
			issues = append(issues, importIssue{Feature: index, Field: "geometry.coordinates[2]", Code: "INVALID_ALTITUDE", Message: "Altitude must be finite numeric"})
		} else if v, e := alt.Float64(); e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			issues = append(issues, importIssue{Feature: index, Field: "geometry.coordinates[2]", Code: "INVALID_ALTITUDE", Message: "Altitude must be finite numeric"})
		} else {
			warnings = append(warnings, importWarning{Feature: index, Field: "geometry.coordinates[2]", Code: "ALTITUDE_DISCARDED", Message: "Altitude is ignored by the two-dimensional entity model"})
		}
	}
	stringProp := func(key string) string {
		v, ok := props[key].(string)
		if !ok {
			issues = append(issues, importIssue{Feature: index, Field: "properties." + key, Code: "REQUIRED_PROPERTY", Message: "Property must be a string"})
			return ""
		}
		return v
	}
	name, typ, status := stringProp("name"), stringProp("type"), stringProp("status")
	description := ""
	if rawDescription, exists := props["description"]; exists {
		var ok bool
		description, ok = rawDescription.(string)
		if !ok {
			issues = append(issues, importIssue{Feature: index, Field: "properties.description", Code: "INVALID_PROPERTY", Message: "description must be a string"})
		}
	}
	input := entity.Input{Name: name, Type: typ, Description: description, Status: status, Latitude: lat, Longitude: lon}
	if err := input.NormalizeAndValidate(); err != nil {
		issues = append(issues, importIssue{Feature: index, Field: "properties", Code: "VALIDATION_ERROR", Message: err.Error()})
	}
	sourceID := ""
	if rawID, exists := feature["id"]; exists {
		var ok bool
		sourceID, ok = rawID.(string)
		if !ok {
			issues = append(issues, importIssue{Feature: index, Field: "id", Code: "INVALID_ID", Message: "Feature id must be a UUID string"})
		}
	}
	id := sourceID
	if idMode == "regenerate" || id == "" {
		id = stableImportID(fingerprint, index)
	} else if uuid.Validate(id) != nil {
		issues = append(issues, importIssue{Feature: index, Field: "id", Code: "INVALID_ID", Message: "Feature id must be a UUID string"})
	}
	if idMode == "regenerate" && sourceID != "" {
		warnings = append(warnings, importWarning{Feature: index, Field: "id", Code: "ID_REGENERATED", Message: "Source ID will be replaced"})
	}
	now := time.Now().UTC()
	created, updated := now, now
	createdValue, createdExists := props["created_at"]
	updatedValue, updatedExists := props["updated_at"]
	createdRaw, cok := createdValue.(string)
	updatedRaw, uok := updatedValue.(string)
	if createdExists || updatedExists {
		if (createdExists && !cok) || (updatedExists && !uok) {
			issues = append(issues, importIssue{Feature: index, Field: "properties.created_at", Code: "INVALID_TIMESTAMP", Message: "Timestamps must be strings"})
		}
		if !cok || !uok {
			issues = append(issues, importIssue{Feature: index, Field: "properties.created_at", Code: "INVALID_TIMESTAMP", Message: "Both timestamps must be supplied"})
		} else {
			var err error
			created, err = time.Parse(time.RFC3339, createdRaw)
			if err != nil {
				issues = append(issues, importIssue{Feature: index, Field: "properties.created_at", Code: "INVALID_TIMESTAMP", Message: "Timestamp must be RFC3339"})
			}
			updated, err = time.Parse(time.RFC3339, updatedRaw)
			if err != nil {
				issues = append(issues, importIssue{Feature: index, Field: "properties.updated_at", Code: "INVALID_TIMESTAMP", Message: "Timestamp must be RFC3339"})
			}
		}
	}
	attrs, err := objectProperty(props, "attributes", maxImportAttributes)
	if err != nil {
		issues = append(issues, importIssue{Feature: index, Field: "properties.attributes", Code: "INVALID_METADATA", Message: err.Error()})
	}
	previous, err := objectProperty(props, "previous_type_values", maxImportMetadata)
	if err != nil {
		issues = append(issues, importIssue{Feature: index, Field: "properties.previous_type_values", Code: "INVALID_METADATA", Message: err.Error()})
	}
	tags, err := stringArrayProperty(props, "tags")
	if err != nil {
		issues = append(issues, importIssue{Feature: index, Field: "properties.tags", Code: "INVALID_METADATA", Message: err.Error()})
	} else {
		warnings = append(warnings, duplicateCatalogWarnings(tags, "properties.tags", 0, index)...)
	}
	categories, err := stringArrayProperty(props, "categories")
	if err != nil {
		issues = append(issues, importIssue{Feature: index, Field: "properties.categories", Code: "INVALID_METADATA", Message: err.Error()})
	} else {
		warnings = append(warnings, duplicateCatalogWarnings(categories, "properties.categories", 0, index)...)
	}
	combinedMetadataSize := 0
	for _, key := range []string{"previous_type_values", "tags", "categories"} {
		if value, exists := props[key]; exists {
			if encoded, marshalErr := json.Marshal(value); marshalErr == nil {
				combinedMetadataSize += len(encoded)
			}
		}
	}
	if combinedMetadataSize > maxImportMetadata {
		issues = append(issues, importIssue{Feature: index, Field: "properties", Code: "METADATA_TOO_LARGE", Message: "Previous values, tags, and categories exceed 64 KiB combined"})
	}
	entityValue := entity.Entity{ID: id, Name: input.Name, Type: input.Type, Description: description, Status: input.Status, Latitude: input.Latitude, Longitude: input.Longitude, CreatedAt: created, UpdatedAt: updated}
	return repository.ImportRecord{Entity: entityValue, Attributes: attrs, PreviousTypeValues: previous, Tags: tags, Categories: categories, SourceID: sourceID, SourceIndex: index}, warnings, issues
}

func sortedJSONMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func duplicateCatalogWarnings(values []string, field string, row, feature int) []importWarning {
	seen := map[string]bool{}
	out := []importWarning{}
	for _, value := range values {
		normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
		if normalized == "" || !seen[normalized] {
			seen[normalized] = true
			continue
		}
		out = append(out, importWarning{Row: row, Feature: feature, Field: field, Code: "DUPLICATE_CATALOG_NAME", Message: "Repeated normalized catalog names will be assigned once"})
	}
	return out
}
func objectProperty(props map[string]any, key string, max int) (map[string]any, error) {
	value, ok := props[key]
	if !ok {
		return map[string]any{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > max {
		return nil, errors.New("object exceeds allowed size")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("must be an object")
	}
	normalized, err := normalizeJSONNumbers(object)
	if err != nil {
		return nil, err
	}
	return normalized.(map[string]any), nil
}
func stringArrayProperty(props map[string]any, key string) ([]string, error) {
	value, ok := props[key]
	if !ok {
		return []string{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > maxImportMetadata {
		return nil, errors.New("array exceeds allowed size")
	}
	array, ok := value.([]any)
	if !ok {
		return nil, errors.New("must be an array of strings")
	}
	out := make([]string, 0, len(array))
	for _, item := range array {
		s, ok := item.(string)
		if !ok {
			return nil, errors.New("must be an array of strings")
		}
		out = append(out, s)
	}
	return out, nil
}

func (a *API) previewImport(w http.ResponseWriter, r *http.Request, store exchangeStore, format, idMode, fingerprint string, parsed []parsedImport) {
	records := []repository.ImportRecord{}
	for _, item := range parsed {
		if item.Record.Entity.ID != "" {
			records = append(records, item.Record)
		}
	}
	preflight, err := store.PreflightImport(r.Context(), records)
	if err != nil {
		a.internalError(w, err)
		return
	}
	preview := importPreview{Format: format, TotalRows: len(parsed), Warnings: []importWarning{}, Errors: []importIssue{}, DuplicateCandidates: []map[string]any{}, ExistingIDs: preflight.ExistingIDs, NewCatalogItems: []map[string]string{}, IDMap: []map[string]string{}, PreviewSHA256: fingerprint, IDMode: idMode}
	issuesByIndex := map[int][]repository.ImportRowIssue{}
	for _, issue := range preflight.Issues {
		issuesByIndex[issue.Index] = append(issuesByIndex[issue.Index], issue)
	}
	for _, item := range parsed {
		preview.Warnings = append(preview.Warnings, item.Warnings...)
		if item.Record.SourceID != "" && item.Record.SourceID != item.Record.Entity.ID {
			preview.IDMap = append(preview.IDMap, map[string]string{"source_id": item.Record.SourceID, "entity_id": item.Record.Entity.ID})
		}
		rowIssues := append([]importIssue{}, item.Issues...)
		for _, issue := range issuesByIndex[item.Index] {
			rowIssues = append(rowIssues, importIssue{Row: rowLocation(format, item.Index), Feature: featureLocation(format, item.Index), Field: issue.Field, Code: issue.Code, Message: issue.Message})
		}
		if item.Record.Entity.ID == "" || len(rowIssues) > 0 {
			preview.InvalidRows++
			preview.Errors = append(preview.Errors, rowIssues...)
		} else {
			preview.ValidRows++
		}
		for _, id := range preflight.DuplicateNames[item.Index] {
			preview.DuplicateCandidates = append(preview.DuplicateCandidates, map[string]any{"row": rowLocation(format, item.Index), "feature": featureLocation(format, item.Index), "entity_id": id, "reason": "duplicate normalized name"})
		}
	}
	kinds := make([]string, 0, len(preflight.NewCatalogItems))
	for kind := range preflight.NewCatalogItems {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		names := preflight.NewCatalogItems[kind]
		for _, name := range names {
			preview.NewCatalogItems = append(preview.NewCatalogItems, map[string]string{"kind": kind, "name": name})
			preview.Warnings = append(preview.Warnings, importWarning{Code: "UNKNOWN_" + strings.ToUpper(kind), Message: name + " will be created on commit"})
		}
	}
	if len(preview.DuplicateCandidates) > maxImportIssues {
		preview.DuplicateCandidates = preview.DuplicateCandidates[:maxImportIssues]
		preview.DuplicateCandidatesTruncated = true
	}
	if len(preview.ExistingIDs) > maxImportIssues {
		preview.ExistingIDs = preview.ExistingIDs[:maxImportIssues]
		preview.ExistingIDsTruncated = true
	}
	if len(preview.NewCatalogItems) > maxImportIssues {
		preview.NewCatalogItems = preview.NewCatalogItems[:maxImportIssues]
		preview.NewCatalogItemsTruncated = true
	}
	if len(preview.Errors) > maxImportIssues {
		preview.Errors = preview.Errors[:maxImportIssues]
		preview.ErrorsTruncated = true
	}
	if len(preview.Warnings) > maxImportIssues {
		preview.Warnings = preview.Warnings[:maxImportIssues]
		preview.WarningsTruncated = true
	}
	writeJSON(w, 200, map[string]any{"data": preview})
}
func rowLocation(format string, index int) int {
	if format == "csv" {
		return index
	}
	return 0
}
func featureLocation(format string, index int) int {
	if format == "geojson" {
		return index
	}
	return 0
}

func (a *API) commitImport(w http.ResponseWriter, r *http.Request, store exchangeStore, format, idMode, fingerprint string, parsed []parsedImport) {
	records := make([]repository.ImportRecord, 0, len(parsed))
	idMap := []map[string]string{}
	for _, item := range parsed {
		if item.Record.Entity.ID == "" || len(item.Issues) > 0 {
			details := []importIssue{}
			for _, row := range item.Issues {
				row.Row, row.Feature = rowLocation(format, item.Index), featureLocation(format, item.Index)
				details = append(details, row)
			}
			writeImportError(w, 400, "INVALID_IMPORT", "Import contains invalid records", details)
			return
		}
		records = append(records, item.Record)
		if item.Record.SourceID != "" && item.Record.SourceID != item.Record.Entity.ID {
			idMap = append(idMap, map[string]string{"source_id": item.Record.SourceID, "entity_id": item.Record.Entity.ID})
		}
	}
	preflight, err := store.PreflightImport(r.Context(), records)
	if err != nil {
		a.internalError(w, err)
		return
	}
	if len(preflight.Issues) > 0 {
		details := make([]importIssue, 0, len(preflight.Issues))
		conflict := false
		for _, row := range preflight.Issues {
			if row.Code == "ID_EXISTS" {
				conflict = true
			}
			details = append(details, importIssue{Row: rowLocation(format, row.Index), Feature: featureLocation(format, row.Index), Field: row.Field, Code: row.Code, Message: row.Message})
		}
		if conflict {
			writeImportError(w, 409, "IMPORT_CONFLICT", "Import conflicts with current entity state", details)
		} else {
			writeImportError(w, 409, "PREVIEW_STALE", "Entity metadata changed after preview", details)
		}
		return
	}
	batchID := uuid.NewString()
	tags, categories, err := store.CreateImportedEntities(r.Context(), format, batchID, records)
	if errors.Is(err, repository.ErrImportConflict) {
		writeError(w, 409, "IMPORT_CONFLICT", "An entity ID was created after preview; no records were imported", "")
		return
	}
	if errors.Is(err, repository.ErrImportInvalid) {
		writeError(w, 409, "PREVIEW_STALE", "Attribute definitions changed after preview", "")
		return
	}
	if errors.Is(err, repository.ErrActivityPayloadTooLarge) {
		writeError(w, 413, "IMPORT_LIMIT_EXCEEDED", "Activity payload exceeds the supported size", "")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"data": importResult{BatchID: batchID, Format: format, Imported: len(records), Created: len(records), TagsCreated: tags, CategoriesCreated: categories, IDMap: idMap}})
}
