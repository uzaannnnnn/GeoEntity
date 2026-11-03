package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"geoentity/backend/internal/entity"

	"github.com/google/uuid"
)

var (
	ErrInvalidQuery   = errors.New("invalid entity query")
	ErrInvalidCursor  = errors.New("invalid entity cursor")
	ErrInvalidBounds  = errors.New("invalid map bounds")
	ErrMapResultLimit = errors.New("map result limit exceeded")
)

const (
	DefaultEntityPageSize = 50
	MaxEntityPageSize     = 100
	MaxMapFeatures        = 5000
	maxQueryLength        = 128
	maxCursorLength       = 1024
)

type EntityFilters struct {
	Q      string
	Type   string
	Status string
}

type EntityQuery struct {
	EntityFilters
	Limit  int
	Cursor string
}

type EntityPageMeta struct {
	Limit         int            `json:"limit"`
	NextCursor    string         `json:"next_cursor"`
	MatchingCount int            `json:"matching_count"`
	StatusCounts  map[string]int `json:"status_counts"`
}

type EntityPage struct {
	Data []entity.Entity `json:"data"`
	Meta EntityPageMeta  `json:"meta"`
}

type MapEntity struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type MapBounds struct {
	West  float64 `json:"west"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	North float64 `json:"north"`
}

type MapExtent struct {
	Count  int        `json:"count"`
	Bounds *MapBounds `json:"bounds"`
}

type entityCursor struct {
	Version    int    `json:"v"`
	CreatedAt  string `json:"created_at"`
	ID         string `json:"id"`
	FilterHash string `json:"filter_hash"`
}

type canonicalFilters struct {
	Q      string `json:"q"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Sort   string `json:"sort"`
}

func NormalizeEntityFilters(filters EntityFilters) (EntityFilters, error) {
	if !utf8.ValidString(filters.Q) {
		return EntityFilters{}, ErrInvalidQuery
	}
	filters.Q = normalizeWhitespace(filters.Q)
	if utf8.RuneCountInString(filters.Q) > maxQueryLength {
		return EntityFilters{}, ErrInvalidQuery
	}
	if filters.Type != "" && !validEntityType(filters.Type) {
		return EntityFilters{}, ErrInvalidQuery
	}
	if filters.Status != "" && !validEntityStatus(filters.Status) {
		return EntityFilters{}, ErrInvalidQuery
	}
	return filters, nil
}

func normalizeWhitespace(value string) string {
	var b strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if b.Len() > 0 {
				space = true
			}
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validEntityType(value string) bool {
	switch value {
	case "vehicle", "iot_device", "facility", "other":
		return true
	default:
		return false
	}
}

func validEntityStatus(value string) bool {
	switch value {
	case "active", "inactive", "maintenance":
		return true
	default:
		return false
	}
}

func (r *SQLite) QueryEntities(ctx context.Context, query EntityQuery) (EntityPage, error) {
	filters, err := NormalizeEntityFilters(query.EntityFilters)
	if err != nil {
		return EntityPage{}, err
	}
	if query.Limit == 0 {
		query.Limit = DefaultEntityPageSize
	}
	if query.Limit < 1 || query.Limit > MaxEntityPageSize {
		return EntityPage{}, ErrInvalidQuery
	}
	filterHash := entityFilterHash(filters)
	var cursor *entityCursor
	if query.Cursor != "" {
		cursor, err = decodeEntityCursor(query.Cursor)
		if err != nil || cursor.FilterHash != filterHash {
			return EntityPage{}, ErrInvalidCursor
		}
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return EntityPage{}, err
	}
	defer tx.Rollback()

	where, args := entityWhere("e", filters, true)
	countQuery := `SELECT COUNT(*) FROM entities e` + where
	var matchingCount int
	if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&matchingCount); err != nil {
		return EntityPage{}, err
	}

	statusWhere, statusArgs := entityWhere("e", filters, false)
	var active, inactive, maintenance int
	statusQuery := `SELECT
		COALESCE(SUM(CASE WHEN e.status='active' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN e.status='inactive' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN e.status='maintenance' THEN 1 ELSE 0 END),0)
		FROM entities e` + statusWhere
	if err := tx.QueryRowContext(ctx, statusQuery, statusArgs...).Scan(&active, &inactive, &maintenance); err != nil {
		return EntityPage{}, err
	}

	pageWhere, pageArgs := entityWhere("e", filters, true)
	if cursor != nil {
		pageWhere += ` AND (e.created_at > ? OR (e.created_at = ? AND e.id > ?))`
		pageArgs = append(pageArgs, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	pageArgs = append(pageArgs, query.Limit+1)
	pageSQL := `SELECT e.id,e.name,e.type,e.description,e.status,e.latitude,e.longitude,e.created_at,e.updated_at
		FROM entities e` + pageWhere + ` ORDER BY e.created_at ASC,e.id ASC LIMIT ?`
	rows, err := tx.QueryContext(ctx, pageSQL, pageArgs...)
	if err != nil {
		return EntityPage{}, err
	}
	items := make([]entity.Entity, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			rows.Close()
			return EntityPage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return EntityPage{}, err
	}
	if err := rows.Close(); err != nil {
		return EntityPage{}, err
	}
	next := ""
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1]
		next, err = encodeEntityCursor(entityCursor{Version: 1, CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID, FilterHash: filterHash})
		if err != nil {
			return EntityPage{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return EntityPage{}, err
	}
	return EntityPage{
		Data: items,
		Meta: EntityPageMeta{
			Limit: query.Limit, NextCursor: next, MatchingCount: matchingCount,
			StatusCounts: map[string]int{"active": active, "inactive": inactive, "maintenance": maintenance},
		},
	}, nil
}

func (r *SQLite) MapEntities(ctx context.Context, filters EntityFilters, bounds MapBounds) ([]MapEntity, error) {
	filters, err := NormalizeEntityFilters(filters)
	if err != nil {
		return nil, err
	}
	if !validBounds(bounds) {
		return nil, ErrInvalidBounds
	}
	where, args := entityWhere("e", filters, true)
	if bounds.West <= bounds.East {
		where += ` AND e.longitude >= ? AND e.longitude <= ?`
		args = append(args, bounds.West, bounds.East)
	} else {
		where += ` AND (e.longitude >= ? OR e.longitude <= ?)`
		args = append(args, bounds.West, bounds.East)
	}
	where += ` AND e.latitude >= ? AND e.latitude <= ?`
	args = append(args, bounds.South, bounds.North, MaxMapFeatures+1)
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.name,e.type,e.status,e.latitude,e.longitude FROM entities e`+where+` ORDER BY e.id ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MapEntity, 0, MaxMapFeatures)
	for rows.Next() {
		var item MapEntity
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Status, &item.Latitude, &item.Longitude); err != nil {
			return nil, err
		}
		items = append(items, item)
		if len(items) > MaxMapFeatures {
			return nil, ErrMapResultLimit
		}
	}
	return items, rows.Err()
}

func (r *SQLite) MapEntityExtent(ctx context.Context, filters EntityFilters) (MapExtent, error) {
	filters, err := NormalizeEntityFilters(filters)
	if err != nil {
		return MapExtent{}, err
	}
	where, args := entityWhere("e", filters, true)
	var count int
	var west, south, east, north sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*),MIN(e.longitude),MIN(e.latitude),MAX(e.longitude),MAX(e.latitude) FROM entities e`+where, args...).Scan(&count, &west, &south, &east, &north)
	if err != nil {
		return MapExtent{}, err
	}
	result := MapExtent{Count: count}
	if count > 0 && west.Valid && south.Valid && east.Valid && north.Valid {
		result.Bounds = &MapBounds{West: west.Float64, South: south.Float64, East: east.Float64, North: north.Float64}
	}
	return result, nil
}

func validBounds(bounds MapBounds) bool {
	return finite(bounds.West) && finite(bounds.East) && finite(bounds.South) && finite(bounds.North) &&
		bounds.West >= -180 && bounds.West <= 180 && bounds.East >= -180 && bounds.East <= 180 &&
		bounds.South >= -90 && bounds.South <= 90 && bounds.North >= -90 && bounds.North <= 90 && bounds.South <= bounds.North
}

func finite(value float64) bool {
	return value == value && value < 1.7976931348623157e308 && value > -1.7976931348623157e308
}

func entityWhere(alias string, filters EntityFilters, includeStatus bool) (string, []any) {
	var clauses []string
	var args []any
	if filters.Q != "" {
		if utf8.RuneCountInString(filters.Q) >= 3 {
			clauses = append(clauses, alias+`.rowid IN (SELECT rowid FROM entity_search WHERE entity_search MATCH ?)`)
			args = append(args, quoteFTSPhrase(filters.Q))
		} else {
			clauses = append(clauses, `(instr(lower(`+alias+`.name),lower(?)) > 0 OR instr(lower(`+alias+`.description),lower(?)) > 0)`)
			args = append(args, filters.Q, filters.Q)
		}
	}
	if filters.Type != "" {
		clauses = append(clauses, alias+`.type = ?`)
		args = append(args, filters.Type)
	}
	if includeStatus && filters.Status != "" {
		clauses = append(clauses, alias+`.status = ?`)
		args = append(args, filters.Status)
	}
	if len(clauses) == 0 {
		return ` WHERE 1=1`, args
	}
	return ` WHERE ` + strings.Join(clauses, ` AND `), args
}

func quoteFTSPhrase(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}

func entityFilterHash(filters EntityFilters) string {
	encoded, _ := json.Marshal(canonicalFilters{Q: filters.Q, Type: filters.Type, Status: filters.Status, Sort: "created_at.asc,id.asc.v1"})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func encodeEntityCursor(cursor entityCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeEntityCursor(value string) (*entityCursor, error) {
	if len(value) == 0 || len(value) > maxCursorLength {
		return nil, ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	var cursor entityCursor
	if err := decoder.Decode(&cursor); err != nil {
		return nil, ErrInvalidCursor
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, ErrInvalidCursor
	}
	if cursor.Version != 1 || cursor.FilterHash == "" || cursor.CreatedAt == "" || uuid.Validate(cursor.ID) != nil {
		return nil, ErrInvalidCursor
	}
	if len(cursor.FilterHash) != sha256.Size*2 {
		return nil, ErrInvalidCursor
	}
	if _, err := hex.DecodeString(cursor.FilterHash); err != nil {
		return nil, ErrInvalidCursor
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt); err != nil {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}
