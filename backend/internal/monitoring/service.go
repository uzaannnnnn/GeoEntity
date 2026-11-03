package monitoring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("device not authorized")
	ErrConflict     = errors.New("telemetry conflict")
	ErrUnitConflict = errors.New("sensor unit conflict")
	ErrInactive     = errors.New("entity not active")
	ErrInvalid      = errors.New("invalid observation")
	sensorPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	allowedUnits    = map[string]bool{"Cel": true, "%": true, "V": true, "A": true, "m": true, "Pa": true, "ppm": true, "dBm": true, "1": true}
)

type Service struct {
	DB  *sql.DB
	now func() time.Time
}

func New(db *sql.DB) *Service { return &Service{DB: db, now: time.Now} }

type Device struct {
	ID               string  `json:"id"`
	EntityID         *string `json:"entity_id"`
	Label            string  `json:"label"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	LastSeenAt       *string `json:"last_seen_at"`
	ExpectedInterval *int    `json:"expected_interval_seconds"`
}
type Observation struct {
	ID         string  `json:"id"`
	EntityID   string  `json:"entity_id"`
	SensorID   string  `json:"sensor_id"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	ObservedAt string  `json:"observed_at"`
	IngestedAt string  `json:"ingested_at"`
	Duplicate  bool    `json:"duplicate"`
}
type Input struct {
	SensorID   string         `json:"sensor_id"`
	Value      float64        `json:"value"`
	Unit       string         `json:"unit"`
	ObservedAt string         `json:"observed_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func (s *Service) CreateCredential(ctx context.Context, entityID, label string, interval *int) (Device, string, error) {
	if strings.TrimSpace(label) == "" || len(label) > 120 {
		return Device{}, "", ErrInvalid
	}
	if interval != nil && (*interval < 1 || *interval > 31536000) {
		return Device{}, "", ErrInvalid
	}
	var status string
	if err := s.DB.QueryRowContext(ctx, `SELECT status FROM entities WHERE id=?`, entityID).Scan(&status); err != nil {
		return Device{}, "", err
	}
	if status != "active" {
		return Device{}, "", ErrInactive
	}
	id, credID := uuid.NewString(), uuid.NewString()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return Device{}, "", err
	}
	h := sha256.Sum256(secret)
	now := s.now().UTC().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO devices(id,entity_id,label,status,expected_interval_seconds,created_at,updated_at) VALUES(?,?,?,'active',?,?,?)`, id, entityID, label, interval, now, now); err != nil {
		return Device{}, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO device_credentials(id,device_id,secret_hash,created_at) VALUES(?,?,?,?)`, credID, id, hex.EncodeToString(h[:]), now); err != nil {
		return Device{}, "", err
	}
	if err = txAudit(ctx, tx, "device.provisioned", id, map[string]any{"entity_id": entityID}); err != nil {
		return Device{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Device{}, "", err
	}
	token := "gdev_" + credID + "." + base64.RawURLEncoding.EncodeToString(secret)
	for i := range secret {
		secret[i] = 0
	}
	return Device{ID: id, EntityID: &entityID, Label: label, Status: "active", CreatedAt: now, ExpectedInterval: interval}, token, nil
}
func (s *Service) ListDevices(ctx context.Context, limit int, before string) ([]Device, error) {
	if limit < 1 || limit > 201 {
		return nil, ErrInvalid
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,entity_id,label,status,created_at,last_seen_at,expected_interval_seconds FROM devices WHERE (?='' OR id>?) ORDER BY id LIMIT ?`, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		var entityID, last sql.NullString
		var interval sql.NullInt64
		if err = rows.Scan(&d.ID, &entityID, &d.Label, &d.Status, &d.CreatedAt, &last, &interval); err != nil {
			return nil, err
		}
		if entityID.Valid {
			d.EntityID = &entityID.String
		}
		if last.Valid {
			d.LastSeenAt = &last.String
		}
		if interval.Valid {
			v := int(interval.Int64)
			d.ExpectedInterval = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Service) Rotate(ctx context.Context, id string) (string, error) {
	return s.mutateCredential(ctx, id, "device.credential_rotated", true)
}
func (s *Service) Revoke(ctx context.Context, id string) error {
	_, err := s.mutateCredential(ctx, id, "device.revoked", false)
	return err
}
func (s *Service) mutateCredential(ctx context.Context, id, action string, rotate bool) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var status string
	var entity sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT status,entity_id FROM devices WHERE id=?`, id).Scan(&status, &entity); err != nil {
		return "", err
	}
	if status != "active" || !entity.Valid {
		return "", ErrInactive
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `UPDATE device_credentials SET revoked_at=? WHERE device_id=? AND revoked_at IS NULL`, now, id); err != nil {
		return "", err
	}
	token := ""
	if rotate {
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			return "", err
		}
		cid := uuid.NewString()
		h := sha256.Sum256(raw)
		if _, err = tx.ExecContext(ctx, `INSERT INTO device_credentials(id,device_id,secret_hash,created_at) VALUES(?,?,?,?)`, cid, id, hex.EncodeToString(h[:]), now); err != nil {
			return "", err
		}
		token = "gdev_" + cid + "." + base64.RawURLEncoding.EncodeToString(raw)
		for i := range raw {
			raw[i] = 0
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE devices SET status='revoked',revoked_at=?,updated_at=? WHERE id=?`, now, now, id); err != nil {
			return "", err
		}
	}
	if err = txAudit(ctx, tx, action, id, map[string]any{"entity_id": entity.String}); err != nil {
		return "", err
	}
	return token, tx.Commit()
}

func (s *Service) Detach(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now().UTC().Format(time.RFC3339Nano)
	var entity sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT entity_id FROM devices WHERE id=?`, id).Scan(&entity); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE device_credentials SET revoked_at=? WHERE device_id=? AND revoked_at IS NULL`, now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE devices SET entity_id=NULL,status='detached',revoked_at=?,updated_at=? WHERE id=?`, now, now, id); err != nil {
		return err
	}
	if err = txAudit(ctx, tx, "device.detached", id, map[string]any{"entity_id": entity.String}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Reassociate(ctx context.Context, id, entityID string) (string, error) {
	var status string
	if err := s.DB.QueryRowContext(ctx, `SELECT status FROM entities WHERE id=?`, entityID).Scan(&status); err != nil {
		return "", err
	}
	if status != "active" {
		return "", ErrInactive
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var old sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT entity_id FROM devices WHERE id=?`, id).Scan(&old); err != nil {
		return "", err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `UPDATE device_credentials SET revoked_at=? WHERE device_id=? AND revoked_at IS NULL`, now, id); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE devices SET entity_id=?,status='active',revoked_at=NULL,updated_at=? WHERE id=?`, entityID, now, id); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	cid := uuid.NewString()
	h := sha256.Sum256(raw)
	if _, err = tx.ExecContext(ctx, `INSERT INTO device_credentials(id,device_id,secret_hash,created_at) VALUES(?,?,?,?)`, cid, id, hex.EncodeToString(h[:]), now); err != nil {
		return "", err
	}
	if err = txAudit(ctx, tx, "device.reassociated", id, map[string]any{"previous_entity_id": old.String, "entity_id": entityID}); err != nil {
		return "", err
	}
	token := "gdev_" + cid + "." + base64.RawURLEncoding.EncodeToString(raw)
	for i := range raw {
		raw[i] = 0
	}
	return token, tx.Commit()
}

func (s *Service) Authenticate(ctx context.Context, bearer string) (string, string, string, error) { // Strict credential shape; the token itself is never included in errors.
	if !strings.HasPrefix(bearer, "gdev_") || strings.Count(bearer, ".") != 1 {
		return "", "", "", ErrUnauthorized
	}
	parts := strings.SplitN(strings.TrimPrefix(bearer, "gdev_"), ".", 2)
	if len(parts) != 2 {
		return "", "", "", ErrUnauthorized
	}
	parsedID, err := uuid.Parse(parts[0])
	if err != nil || parsedID.String() != parts[0] {
		return "", "", "", ErrUnauthorized
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(secret) != 32 {
		return "", "", "", ErrUnauthorized
	}
	got := sha256.Sum256(secret)
	for i := range secret {
		secret[i] = 0
	}
	var deviceID, stored, entityID, status, entityStatus string
	err = s.DB.QueryRowContext(ctx, `SELECT c.device_id,c.secret_hash,d.entity_id,d.status,e.status FROM device_credentials c JOIN devices d ON d.id=c.device_id JOIN entities e ON e.id=d.entity_id WHERE c.id=? AND c.revoked_at IS NULL`, parts[0]).Scan(&deviceID, &stored, &entityID, &status, &entityStatus)
	if err != nil || status != "active" {
		return "", "", "", ErrUnauthorized
	}
	want, err := hex.DecodeString(stored)
	if err != nil || len(want) != 32 || subtle.ConstantTimeCompare(got[:], want) != 1 {
		return "", "", "", ErrUnauthorized
	}
	if entityStatus != "active" {
		return deviceID, entityID, parts[0], ErrInactive
	}
	return deviceID, entityID, parts[0], nil
}

func (s *Service) Ingest(ctx context.Context, deviceID, entityID, credentialID, key string, in Input) (Observation, bool, error) {
	if len(key) < 1 || len(key) > 128 || !ascii(key) || !sensorPattern.MatchString(in.SensorID) || !allowedUnits[in.Unit] || math.IsNaN(in.Value) || math.IsInf(in.Value, 0) || math.Abs(in.Value) > 1e100 {
		return Observation{}, false, ErrInvalid
	}
	obs, err := time.Parse(time.RFC3339, in.ObservedAt)
	if err != nil {
		return Observation{}, false, ErrInvalid
	}
	obs = obs.UTC()
	now := s.now().UTC()
	if obs.After(now.Add(5*time.Minute)) || obs.Before(now.Add(-90*24*time.Hour)) {
		return Observation{}, false, ErrInvalid
	}
	var metadata any
	var metadataJSON string
	if in.Metadata != nil {
		if !boundedMetadata(in.Metadata, 0) {
			return Observation{}, false, ErrInvalid
		}
		b, e := json.Marshal(in.Metadata)
		if e != nil || len(b) > 1024 || len(in.Metadata) > 32 {
			return Observation{}, false, ErrInvalid
		}
		metadataJSON = string(b)
		metadata = metadataJSON
	}
	canonical, _ := json.Marshal(struct {
		Device, Entity, Sensor   string
		Value                    float64
		Unit, Observed, Metadata string
	}{deviceID, entityID, in.SensorID, in.Value, in.Unit, obs.Format(time.RFC3339Nano), metadataJSON})
	digest := sha256.Sum256(canonical)
	digestHex := hex.EncodeToString(digest[:])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Observation{}, false, err
	}
	defer tx.Rollback()
	var currentEntity, status, entityStatus string
	var credID string
	err = tx.QueryRowContext(ctx, `SELECT d.entity_id,d.status,e.status,c.id FROM devices d JOIN entities e ON e.id=d.entity_id JOIN device_credentials c ON c.device_id=d.id WHERE d.id=? AND c.id=? AND c.revoked_at IS NULL`, deviceID, credentialID).Scan(&currentEntity, &status, &entityStatus, &credID)
	if err != nil || status != "active" || currentEntity != entityID {
		return Observation{}, false, ErrUnauthorized
	}
	if entityStatus != "active" {
		return Observation{}, false, ErrInactive
	}
	var old Observation
	var oldDigest string
	err = tx.QueryRowContext(ctx, `SELECT id,entity_id,sensor_id,value,unit,observed_at,ingested_at,payload_sha256 FROM sensor_observations WHERE device_id=? AND idempotency_key=?`, deviceID, key).Scan(&old.ID, &old.EntityID, &old.SensorID, &old.Value, &old.Unit, &old.ObservedAt, &old.IngestedAt, &oldDigest)
	if err == nil {
		if oldDigest != digestHex {
			return Observation{}, false, ErrConflict
		}
		old.Duplicate = true
		if err = tx.Commit(); err != nil {
			return Observation{}, false, err
		}
		return old, true, nil
	}
	if err != sql.ErrNoRows {
		return Observation{}, false, err
	}
	var unit string
	err = tx.QueryRowContext(ctx, `SELECT unit FROM device_sensors WHERE device_id=? AND sensor_id=?`, deviceID, in.SensorID).Scan(&unit)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, `INSERT INTO device_sensors(device_id,sensor_id,unit,created_at) VALUES(?,?,?,?)`, deviceID, in.SensorID, in.Unit, now.Format(time.RFC3339Nano)); err != nil {
			return Observation{}, false, err
		}
	} else if err != nil {
		return Observation{}, false, err
	} else if unit != in.Unit {
		return Observation{}, false, ErrUnitConflict
	}
	newID := uuid.NewString()
	ingested := now.Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO sensor_observations(id,entity_id,device_id,credential_id,idempotency_key,sensor_id,value,unit,observed_at,ingested_at,payload_sha256,metadata) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, newID, entityID, deviceID, credID, key, in.SensorID, in.Value, in.Unit, obs.Format(time.RFC3339Nano), ingested, digestHex, metadata)
	if err != nil {
		return Observation{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE devices SET last_seen_at=?,updated_at=? WHERE id=? AND (last_seen_at IS NULL OR last_seen_at<=?)`, ingested, ingested, deviceID, now.Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		return Observation{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE device_credentials SET last_used_at=? WHERE id=? AND (last_used_at IS NULL OR last_used_at<=?)`, ingested, credID, now.Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		return Observation{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return Observation{}, false, err
	}
	return Observation{ID: newID, EntityID: entityID, SensorID: in.SensorID, Value: in.Value, Unit: in.Unit, ObservedAt: obs.Format(time.RFC3339Nano), IngestedAt: ingested}, false, nil
}

func boundedMetadata(value any, depth int) bool {
	if depth > 4 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		if len(v) > 32 {
			return false
		}
		for k, child := range v {
			if len(k) > 64 || strings.Contains(strings.ToLower(k), "credential") || strings.Contains(strings.ToLower(k), "secret") || strings.Contains(strings.ToLower(k), "token") {
				return false
			}
			if !boundedMetadata(child, depth+1) {
				return false
			}
		}
	case []any:
		if len(v) > 32 {
			return false
		}
		for _, child := range v {
			if !boundedMetadata(child, depth+1) {
				return false
			}
		}
	}
	return true
}
func ascii(s string) bool {
	for i := range s {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}
func txAudit(ctx context.Context, tx *sql.Tx, action, id string, details any) error {
	b, _ := json.Marshal(details)
	_, err := tx.ExecContext(ctx, `INSERT INTO admin_audit_events(id,occurred_at,actor_id,actor_label,action,resource_type,resource_id,payload_version,details) VALUES(?,?,?,?,?,?,?,1,?)`, uuid.NewString(), time.Now().UTC().Format(time.RFC3339Nano), "1", "administrator", action, "device", id, string(b))
	return err
}
