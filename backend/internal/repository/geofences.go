package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/geofence"

	"github.com/google/uuid"
)

var (
	ErrGeofenceNotFound     = errors.New("geofence not found")
	ErrGeofenceNameConflict = errors.New("geofence name already exists")
	ErrGeofenceLimit        = errors.New("maximum active geofence count reached")
)

const (
	DefaultGeofencePageSize = 50
	MaxGeofencePageSize     = 100
)

type GeofenceFilters struct {
	Enabled *bool
}

type GeofencePage struct {
	Data []geofence.Definition `json:"data"`
	Meta struct {
		Limit      int    `json:"limit"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

type geofenceCursor struct {
	Version    int    `json:"v"`
	CreatedAt  string `json:"created_at"`
	ID         string `json:"id"`
	FilterHash string `json:"filter_hash"`
}

func geofenceFilterHash(filters GeofenceFilters) string {
	value := "enabled=all"
	if filters.Enabled != nil {
		value = fmt.Sprintf("enabled=%t", *filters.Enabled)
	}
	sum := sha256.Sum256([]byte(value + "|sort=created_at,id:v1"))
	return hex.EncodeToString(sum[:])
}

func encodeGeofenceCursor(cursor geofenceCursor) (string, error) {
	body, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodeGeofenceCursor(value string) (geofenceCursor, error) {
	var cursor geofenceCursor
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(body) > 1024 {
		return cursor, ErrInvalidCursor
	}
	if err := json.Unmarshal(body, &cursor); err != nil || cursor.Version != 1 || cursor.CreatedAt == "" || !uuidValid(cursor.ID) {
		return geofenceCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func uuidValid(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func (r *SQLite) Geofences(ctx context.Context, filters GeofenceFilters, limit int, encodedCursor string) (GeofencePage, error) {
	if limit == 0 {
		limit = DefaultGeofencePageSize
	}
	if limit < 1 || limit > MaxGeofencePageSize {
		return GeofencePage{}, ErrInvalidQuery
	}
	filterHash := geofenceFilterHash(filters)
	var cursor *geofenceCursor
	if encodedCursor != "" {
		decoded, err := decodeGeofenceCursor(encodedCursor)
		if err != nil || decoded.FilterHash != filterHash {
			return GeofencePage{}, ErrInvalidCursor
		}
		cursor = &decoded
	}

	where := ""
	args := []any{}
	if filters.Enabled != nil {
		where = " WHERE enabled=?"
		args = append(args, boolInt(*filters.Enabled))
	}
	if cursor != nil {
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += "(created_at>? OR (created_at=? AND id>?))"
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, "SELECT id,name,enabled,geometry_type,geometry_json,created_at,updated_at FROM geofences"+where+" ORDER BY created_at ASC,id ASC LIMIT ?", args...)
	if err != nil {
		return GeofencePage{}, err
	}
	items := make([]geofence.Definition, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanGeofence(rows)
		if scanErr != nil {
			rows.Close()
			return GeofencePage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return GeofencePage{}, err
	}
	if err := rows.Close(); err != nil {
		return GeofencePage{}, err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next, err = encodeGeofenceCursor(geofenceCursor{Version: 1, CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID, FilterHash: filterHash})
		if err != nil {
			return GeofencePage{}, err
		}
	}
	page := GeofencePage{Data: items}
	page.Meta.Limit = limit
	page.Meta.NextCursor = next
	return page, nil
}

func (r *SQLite) GetGeofence(ctx context.Context, id string) (geofence.Definition, error) {
	item, err := scanGeofence(r.db.QueryRowContext(ctx, "SELECT id,name,enabled,geometry_type,geometry_json,created_at,updated_at FROM geofences WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return geofence.Definition{}, ErrGeofenceNotFound
	}
	return item, err
}

func (r *SQLite) CreateGeofence(ctx context.Context, item geofence.Definition) error {
	display, normalized, err := geofence.NormalizeName(item.Name)
	if err != nil {
		return err
	}
	if !uuidValid(item.ID) || geofence.ValidateGeometry(item.Geometry) != nil {
		return geofence.ErrInvalidGeometry
	}
	body, err := json.Marshal(item.Geometry)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM geofences WHERE normalized_name=?", normalized).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return ErrGeofenceNameConflict
	}
	if item.Enabled {
		if err := ensureActiveGeofenceCapacity(ctx, tx, ""); err != nil {
			return err
		}
	}
	now := item.CreatedAt.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, "INSERT INTO geofences(id,name,normalized_name,enabled,geometry_type,geometry_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", item.ID, display, normalized, boolInt(item.Enabled), item.Geometry.Type, string(body), now, item.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return ErrGeofenceNameConflict
		}
		return err
	}
	if err := establishGeofenceMembershipsTx(ctx, tx, item, item.CreatedAt); err != nil {
		return err
	}
	if err := auth.AuditTx(ctx, tx, "geofence.created", "geofence", item.ID, auth.RequestID(ctx), map[string]any{"geometry_type": item.Geometry.Type, "enabled": item.Enabled}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLite) UpdateGeofence(ctx context.Context, item geofence.Definition) error {
	display, normalized, err := geofence.NormalizeName(item.Name)
	if err != nil {
		return err
	}
	if !uuidValid(item.ID) || geofence.ValidateGeometry(item.Geometry) != nil {
		return geofence.ErrInvalidGeometry
	}
	body, err := json.Marshal(item.Geometry)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM geofences WHERE id=?", item.ID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrGeofenceNotFound
	}
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM geofences WHERE normalized_name=? AND id<>?", normalized, item.ID).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return ErrGeofenceNameConflict
	}
	if item.Enabled {
		if err := ensureActiveGeofenceCapacity(ctx, tx, item.ID); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE geofences SET name=?,normalized_name=?,enabled=?,geometry_type=?,geometry_json=?,updated_at=? WHERE id=?", display, normalized, boolInt(item.Enabled), item.Geometry.Type, string(body), item.UpdatedAt.UTC().Format(time.RFC3339Nano), item.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return ErrGeofenceNameConflict
		}
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrGeofenceNotFound
	}
	if item.Enabled {
		if err := establishGeofenceMembershipsTx(ctx, tx, item, item.UpdatedAt); err != nil {
			return err
		}
	} else if err := markGeofenceMembershipsInactiveTx(ctx, tx, item.ID); err != nil {
		return err
	}
	if err := auth.AuditTx(ctx, tx, "geofence.updated", "geofence", item.ID, auth.RequestID(ctx), map[string]any{"geometry_type": item.Geometry.Type, "enabled": item.Enabled}); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureActiveGeofenceCapacity(ctx context.Context, tx *sql.Tx, excludeID string) error {
	var active int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM geofences WHERE enabled=1 AND id<>?", excludeID).Scan(&active); err != nil {
		return err
	}
	if active >= geofence.MaxActiveGeofences {
		return ErrGeofenceLimit
	}
	return nil
}

func (r *SQLite) DeleteGeofence(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "DELETE FROM geofences WHERE id=?", id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrGeofenceNotFound
	}
	if err := auth.AuditTx(ctx, tx, "geofence.deleted", "geofence", id, auth.RequestID(ctx), map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

func scanGeofence(scanner interface{ Scan(...any) error }) (geofence.Definition, error) {
	var item geofence.Definition
	var enabled int
	var geometryType, geometryJSON, created, updated string
	if err := scanner.Scan(&item.ID, &item.Name, &enabled, &geometryType, &geometryJSON, &created, &updated); err != nil {
		return geofence.Definition{}, err
	}
	item.Enabled = enabled == 1
	if err := json.Unmarshal([]byte(geometryJSON), &item.Geometry); err != nil {
		return geofence.Definition{}, err
	}
	if item.Geometry.Type != geometryType {
		return geofence.Definition{}, errors.New("stored geofence geometry type mismatch")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return geofence.Definition{}, errors.New("stored geofence has invalid created_at")
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return geofence.Definition{}, errors.New("stored geofence has invalid updated_at")
	}
	item.CreatedAt, item.UpdatedAt = createdAt, updatedAt
	if err := geofence.ValidateCanonical(item.Geometry); err != nil {
		return geofence.Definition{}, errors.New("stored geofence has invalid geometry")
	}
	return item, nil
}
