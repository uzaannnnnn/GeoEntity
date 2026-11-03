package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/geofence"

	"github.com/google/uuid"
)

const (
	DefaultGeofenceEventPageSize = 50
	MaxGeofenceEventPageSize     = 100
	storedTimestampLayout        = "2006-01-02T15:04:05.000000000Z07:00"
)

func formatStoredTimestamp(value time.Time) string { return value.UTC().Format(storedTimestampLayout) }

type EntityGeofenceMembership struct {
	GeofenceID          string    `json:"geofence_id"`
	GeofenceName        string    `json:"geofence_name"`
	State               string    `json:"state"`
	EvaluatedLatitude   float64   `json:"evaluated_latitude"`
	EvaluatedLongitude  float64   `json:"evaluated_longitude"`
	EvaluatedAt         time.Time `json:"evaluated_at"`
	DefinitionUpdatedAt time.Time `json:"definition_updated_at"`
}

type GeofenceEvent struct {
	ID           string    `json:"id"`
	EntityID     string    `json:"entity_id"`
	EntityName   string    `json:"entity_name"`
	GeofenceID   string    `json:"geofence_id"`
	GeofenceName string    `json:"geofence_name"`
	EventType    string    `json:"event_type"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	OccurredAt   time.Time `json:"occurred_at"`
	Source       string    `json:"source"`
}

type GeofenceEventFilters struct {
	EntityID string
	From     *time.Time
	To       *time.Time
}

type GeofenceEventPage struct {
	Data []GeofenceEvent `json:"data"`
	Meta struct {
		Limit      int    `json:"limit"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

type geofenceEventCursor struct {
	Version    int    `json:"v"`
	OccurredAt string `json:"occurred_at"`
	ID         string `json:"id"`
	FilterHash string `json:"filter_hash"`
}

func eventFilterHash(geofenceID string, filters GeofenceEventFilters) string {
	from, to := "", ""
	if filters.From != nil {
		from = formatStoredTimestamp(*filters.From)
	}
	if filters.To != nil {
		to = formatStoredTimestamp(*filters.To)
	}
	sum := sha256.Sum256([]byte(geofenceID + "|" + filters.EntityID + "|" + from + "|" + to + "|sort=occurred_at,id:desc:v1"))
	return hex.EncodeToString(sum[:])
}

func encodeGeofenceEventCursor(cursor geofenceEventCursor) (string, error) {
	body, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodeGeofenceEventCursor(value string) (geofenceEventCursor, error) {
	var cursor geofenceEventCursor
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(body) > 1024 {
		return cursor, ErrInvalidCursor
	}
	if err := json.Unmarshal(body, &cursor); err != nil || cursor.Version != 1 || cursor.OccurredAt == "" || !uuidValid(cursor.ID) {
		return geofenceEventCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func (r *SQLite) EntityGeofences(ctx context.Context, entityID string) ([]EntityGeofenceMembership, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM entities WHERE id=?)`, entityID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrNotFound
	}
	rows, err := r.db.QueryContext(ctx, `SELECT m.geofence_id,g.name,m.state,m.evaluated_latitude,m.evaluated_longitude,m.evaluated_at,m.definition_updated_at
		FROM entity_geofence_memberships m JOIN geofences g ON g.id=m.geofence_id
		WHERE m.entity_id=? ORDER BY g.name COLLATE NOCASE,g.id`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]EntityGeofenceMembership, 0)
	for rows.Next() {
		item, err := scanEntityGeofenceMembership(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanEntityGeofenceMembership(scanner interface{ Scan(...any) error }) (EntityGeofenceMembership, error) {
	var item EntityGeofenceMembership
	var evaluated, definitionUpdated string
	if err := scanner.Scan(&item.GeofenceID, &item.GeofenceName, &item.State, &item.EvaluatedLatitude, &item.EvaluatedLongitude, &evaluated, &definitionUpdated); err != nil {
		return EntityGeofenceMembership{}, err
	}
	var err error
	item.EvaluatedAt, err = time.Parse(time.RFC3339Nano, evaluated)
	if err != nil {
		return EntityGeofenceMembership{}, errors.New("stored membership has invalid evaluated_at")
	}
	item.DefinitionUpdatedAt, err = time.Parse(time.RFC3339Nano, definitionUpdated)
	if err != nil {
		return EntityGeofenceMembership{}, errors.New("stored membership has invalid definition_updated_at")
	}
	item.EvaluatedAt = item.EvaluatedAt.UTC()
	item.DefinitionUpdatedAt = item.DefinitionUpdatedAt.UTC()
	return item, nil
}

func (r *SQLite) GeofenceEvents(ctx context.Context, geofenceID string, filters GeofenceEventFilters, limit int, encodedCursor string) (GeofenceEventPage, error) {
	if limit == 0 {
		limit = DefaultGeofenceEventPageSize
	}
	if limit < 1 || limit > MaxGeofenceEventPageSize {
		return GeofenceEventPage{}, ErrInvalidQuery
	}
	filterHash := eventFilterHash(geofenceID, filters)
	var cursor *geofenceEventCursor
	if encodedCursor != "" {
		decoded, err := decodeGeofenceEventCursor(encodedCursor)
		if err != nil || decoded.FilterHash != filterHash {
			return GeofenceEventPage{}, ErrInvalidCursor
		}
		cursor = &decoded
	}
	var current int
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM geofences WHERE id=?)`, geofenceID).Scan(&current); err != nil {
		return GeofenceEventPage{}, err
	}
	if current == 0 {
		var historic int
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM geofence_events WHERE geofence_id=?)`, geofenceID).Scan(&historic); err != nil {
			return GeofenceEventPage{}, err
		}
		if historic == 0 {
			return GeofenceEventPage{}, ErrGeofenceNotFound
		}
	}
	where := ` WHERE geofence_id=?`
	args := []any{geofenceID}
	if filters.EntityID != "" {
		where += ` AND entity_id=?`
		args = append(args, filters.EntityID)
	}
	if filters.From != nil {
		where += ` AND occurred_at>=?`
		args = append(args, formatStoredTimestamp(*filters.From))
	}
	if filters.To != nil {
		where += ` AND occurred_at<=?`
		args = append(args, formatStoredTimestamp(*filters.To))
	}
	if cursor != nil {
		where += ` AND (occurred_at<? OR (occurred_at=? AND id<?))`
		args = append(args, cursor.OccurredAt, cursor.OccurredAt, cursor.ID)
	}
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, `SELECT id,entity_id,entity_name,geofence_id,geofence_name,event_type,latitude,longitude,occurred_at,source FROM geofence_events`+where+` ORDER BY occurred_at DESC,id DESC LIMIT ?`, args...)
	if err != nil {
		return GeofenceEventPage{}, err
	}
	items := make([]GeofenceEvent, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanGeofenceEvent(rows)
		if scanErr != nil {
			rows.Close()
			return GeofenceEventPage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return GeofenceEventPage{}, err
	}
	if err := rows.Close(); err != nil {
		return GeofenceEventPage{}, err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next, err = encodeGeofenceEventCursor(geofenceEventCursor{Version: 1, OccurredAt: formatStoredTimestamp(last.OccurredAt), ID: last.ID, FilterHash: filterHash})
		if err != nil {
			return GeofenceEventPage{}, err
		}
	}
	page := GeofenceEventPage{Data: items}
	page.Meta.Limit, page.Meta.NextCursor = limit, next
	return page, nil
}

func scanGeofenceEvent(scanner interface{ Scan(...any) error }) (GeofenceEvent, error) {
	var item GeofenceEvent
	var occurred string
	if err := scanner.Scan(&item.ID, &item.EntityID, &item.EntityName, &item.GeofenceID, &item.GeofenceName, &item.EventType, &item.Latitude, &item.Longitude, &occurred, &item.Source); err != nil {
		return GeofenceEvent{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, occurred)
	if err != nil {
		return GeofenceEvent{}, errors.New("stored geofence event has invalid occurred_at")
	}
	item.OccurredAt = parsed.UTC()
	return item, nil
}

func loadEnabledGeofencesTx(ctx context.Context, tx *sql.Tx) ([]geofence.Definition, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,name,enabled,geometry_type,geometry_json,created_at,updated_at FROM geofences WHERE enabled=1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	items := make([]geofence.Definition, 0)
	for rows.Next() {
		item, scanErr := scanGeofence(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, nil
}

func loadEntityMembershipStatesTx(ctx context.Context, tx *sql.Tx, entityID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT geofence_id,state FROM entity_geofence_memberships WHERE entity_id=?`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := make(map[string]string)
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			return nil, err
		}
		states[id] = state
	}
	return states, rows.Err()
}

func establishEntityMembershipsTx(ctx context.Context, tx *sql.Tx, item entity.Entity, geofences []geofence.Definition, evaluatedAt time.Time) error {
	for _, definition := range geofences {
		if err := saveMembershipTx(ctx, tx, item.ID, definition, item.Latitude, item.Longitude, evaluatedAt, membershipState(definition, item.Latitude, item.Longitude)); err != nil {
			return err
		}
	}
	return nil
}

type membershipEvaluation struct {
	definition  geofence.Definition
	state       string
	eventType   string
	createEvent bool
	verifiedAt  time.Time
}

func prepareEntityMembershipUpdatesTx(ctx context.Context, tx *sql.Tx, previous, item entity.Entity) ([]membershipEvaluation, error) {
	if previous.Latitude == item.Latitude && previous.Longitude == item.Longitude {
		return nil, nil
	}
	definitions, err := loadEnabledGeofencesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	previousStates, err := loadEntityMembershipStatesTx(ctx, tx, item.ID)
	if err != nil {
		return nil, err
	}
	evaluatedAt := time.Now().UTC()
	evaluations := make([]membershipEvaluation, 0, len(definitions))
	for _, definition := range definitions {
		nextState := membershipState(definition, item.Latitude, item.Longitude)
		oldState, exists := previousStates[definition.ID]
		eventType := ""
		if exists && (oldState == "inside" || oldState == "outside") && oldState != nextState {
			eventType = "exit"
			if nextState == "inside" {
				eventType = "enter"
			}
		}
		evaluations = append(evaluations, membershipEvaluation{definition: definition, state: nextState, eventType: eventType, createEvent: eventType != "", verifiedAt: evaluatedAt})
	}
	return evaluations, nil
}

func persistEntityMembershipUpdatesTx(ctx context.Context, tx *sql.Tx, item entity.Entity, evaluations []membershipEvaluation) error {
	for _, evaluation := range evaluations {
		evaluatedAt := evaluation.verifiedAt
		if evaluation.createEvent {
			var err error
			evaluatedAt, err = nextGeofenceEventTimestampTx(ctx, tx, evaluation.definition.ID, evaluatedAt)
			if err != nil {
				return err
			}
		}
		if err := saveMembershipTx(ctx, tx, item.ID, evaluation.definition, item.Latitude, item.Longitude, evaluatedAt, evaluation.state); err != nil {
			return err
		}
		if evaluation.createEvent {
			if err := insertGeofenceTransitionTx(ctx, tx, item, evaluation.definition, evaluation.eventType, evaluatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func nextGeofenceEventTimestampTx(ctx context.Context, tx *sql.Tx, geofenceID string, proposed time.Time) (time.Time, error) {
	var latest string
	err := tx.QueryRowContext(ctx, `SELECT occurred_at FROM geofence_events WHERE geofence_id=? ORDER BY occurred_at DESC,id DESC LIMIT 1`, geofenceID).Scan(&latest)
	if errors.Is(err, sql.ErrNoRows) {
		return proposed, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	previous, err := time.Parse(time.RFC3339Nano, latest)
	if err != nil {
		return time.Time{}, errors.New("stored geofence event has invalid occurred_at")
	}
	if !proposed.After(previous) {
		return previous.Add(time.Nanosecond), nil
	}
	return proposed, nil
}

func membershipState(definition geofence.Definition, latitude, longitude float64) string {
	if definition.Geometry.Contains(latitude, longitude) {
		return "inside"
	}
	return "outside"
}

func saveMembershipTx(ctx context.Context, tx *sql.Tx, entityID string, definition geofence.Definition, latitude, longitude float64, evaluatedAt time.Time, state string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO entity_geofence_memberships(entity_id,geofence_id,state,evaluated_latitude,evaluated_longitude,evaluated_at,definition_updated_at)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(entity_id,geofence_id) DO UPDATE SET state=excluded.state,evaluated_latitude=excluded.evaluated_latitude,evaluated_longitude=excluded.evaluated_longitude,evaluated_at=excluded.evaluated_at,definition_updated_at=excluded.definition_updated_at`,
		entityID, definition.ID, state, latitude, longitude, formatStoredTimestamp(evaluatedAt), definition.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func insertGeofenceTransitionTx(ctx context.Context, tx *sql.Tx, item entity.Entity, definition geofence.Definition, eventType string, occurredAt time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO geofence_events(id,entity_id,entity_name,geofence_id,geofence_name,event_type,latitude,longitude,occurred_at,source)
		VALUES(?,?,?,?,?,?,?,?,?,'coordinate_update')`, uuid.NewString(), item.ID, item.Name, definition.ID, definition.Name, eventType, item.Latitude, item.Longitude, formatStoredTimestamp(occurredAt))
	return err
}

func establishGeofenceMembershipsTx(ctx context.Context, tx *sql.Tx, definition geofence.Definition, evaluatedAt time.Time) error {
	if !definition.Enabled {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,name,type,description,status,latitude,longitude,created_at,updated_at FROM entities ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		if err := saveMembershipTx(ctx, tx, item.ID, definition, item.Latitude, item.Longitude, evaluatedAt, membershipState(definition, item.Latitude, item.Longitude)); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return nil
}

func markGeofenceMembershipsInactiveTx(ctx context.Context, tx *sql.Tx, geofenceID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE entity_geofence_memberships SET state='inactive' WHERE geofence_id=?`, geofenceID)
	return err
}

func (r *SQLite) backfillMissingGeofenceMemberships(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	definitions, err := loadEnabledGeofencesTx(ctx, tx)
	if err != nil {
		return err
	}
	evaluatedAt := time.Now().UTC()
	for _, definition := range definitions {
		rows, err := tx.QueryContext(ctx, `SELECT e.id,e.name,e.type,e.description,e.status,e.latitude,e.longitude,e.created_at,e.updated_at
			FROM entities e LEFT JOIN entity_geofence_memberships m ON m.entity_id=e.id AND m.geofence_id=?
			WHERE m.entity_id IS NULL ORDER BY e.id`, definition.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			item, scanErr := scanEntity(rows)
			if scanErr != nil {
				rows.Close()
				return scanErr
			}
			state := membershipState(definition, item.Latitude, item.Longitude)
			if err := saveMembershipTx(ctx, tx, item.ID, definition, item.Latitude, item.Longitude, evaluatedAt, state); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return tx.Commit()
}
