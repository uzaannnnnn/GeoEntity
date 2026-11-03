package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/entity"
	"github.com/google/uuid"
)

const MaxActivityPayload = 64 << 10
const activityTimestampFormat = "2006-01-02T15:04:05.000000000Z"
const activityDescriptionLimit = 4096
const activityValueTextLimit = 256

var ErrActivityPayloadTooLarge = errors.New("activity payload exceeds 64 KiB")

func activityEntity(item entity.Entity) map[string]any {
	description, truncated := truncateUTF8(item.Description, activityDescriptionLimit)
	return map[string]any{"id": item.ID, "name": item.Name, "type": item.Type, "description": description, "description_truncated": truncated, "status": item.Status, "latitude": item.Latitude, "longitude": item.Longitude, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt}
}
func truncateUTF8(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	end := 0
	for i := range s {
		if i > max {
			break
		}
		end = i
	}
	if end == 0 {
		return "", true
	}
	return s[:end], true
}
func activityAttributeValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for k, v := range values {
		if s, ok := v.(string); ok {
			clipped, truncated := truncateUTF8(s, activityValueTextLimit)
			if truncated {
				out[k] = map[string]any{"value": clipped, "truncated": true}
			} else {
				out[k] = s
			}
		} else {
			out[k] = v
		}
	}
	return out
}

type Definition struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	ValueType  string    `json:"value_type"`
	Required   bool      `json:"required"`
	MinNumber  *float64  `json:"min_number,omitempty"`
	MaxNumber  *float64  `json:"max_number,omitempty"`
	MaxLength  *int      `json:"max_length,omitempty"`
	Archived   bool      `json:"archived"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AdminAuditEvent struct {
	ID           string          `json:"id"`
	OccurredAt   string          `json:"occurred_at"`
	ActorID      string          `json:"actor_id"`
	ActorLabel   string          `json:"actor_label"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	RequestID    string          `json:"request_id"`
	Details      json.RawMessage `json:"details"`
}

func (r *SQLite) ListAdminAudit(ctx context.Context, limit int, beforeTime, beforeID string) ([]AdminAuditEvent, error) {
	if limit < 1 || limit > 101 {
		return nil, errors.New("audit page limit must be between 1 and 101")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,occurred_at,actor_id,actor_label,action,resource_type,resource_id,request_id,details FROM admin_audit_events WHERE (?='' OR occurred_at<? OR (occurred_at=? AND id<?)) ORDER BY occurred_at DESC,id DESC LIMIT ?`, beforeTime, beforeTime, beforeTime, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AdminAuditEvent, 0, limit)
	for rows.Next() {
		var e AdminAuditEvent
		var details string
		if err = rows.Scan(&e.ID, &e.OccurredAt, &e.ActorID, &e.ActorLabel, &e.Action, &e.ResourceType, &e.ResourceID, &e.RequestID, &details); err != nil {
			return nil, err
		}
		e.Details = json.RawMessage(details)
		out = append(out, e)
	}
	return out, rows.Err()
}

type Activity struct {
	ID         string    `json:"id"`
	EntityID   string    `json:"entity_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Actor      struct {
		Kind string  `json:"kind"`
		ID   *string `json:"id"`
	} `json:"actor"`
	PayloadVersion int             `json:"payload_version"`
	Changes        json.RawMessage `json:"changes"`
}
type ActivityPage struct {
	Data       []Activity `json:"data"`
	NextCursor string     `json:"next_cursor"`
}
type cursor struct{ Time, ID string }

func insertActivity(ctx context.Context, tx *sql.Tx, entityID, kind string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if len(body) > MaxActivityPayload {
		return ErrActivityPayloadTooLarge
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_events(id,entity_id,event_type,occurred_at,actor_kind,actor_id,payload_version,payload) VALUES(?,?,?,?, 'system',NULL,1,?)`, uuid.NewString(), entityID, kind, time.Now().UTC().Format(activityTimestampFormat), string(body))
	return err
}
func (r *SQLite) Definitions(ctx context.Context, typ string) ([]Definition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,entity_type,key,label,value_type,required,min_number,max_number,max_length,archived,created_at,updated_at FROM attribute_definitions WHERE entity_type=? ORDER BY key`, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Definition{}
	for rows.Next() {
		d, e := scanDefinition(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *SQLite) Definition(ctx context.Context, id string) (Definition, error) {
	d, err := scanDefinition(r.db.QueryRowContext(ctx, `SELECT id,entity_type,key,label,value_type,required,min_number,max_number,max_length,archived,created_at,updated_at FROM attribute_definitions WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Definition{}, ErrNotFound
	}
	return d, err
}
func scanDefinition(s interface{ Scan(...any) error }) (Definition, error) {
	var d Definition
	var req, arch int
	var created, updated string
	err := s.Scan(&d.ID, &d.EntityType, &d.Key, &d.Label, &d.ValueType, &req, &d.MinNumber, &d.MaxNumber, &d.MaxLength, &arch, &created, &updated)
	if err != nil {
		return d, err
	}
	d.Required = req == 1
	d.Archived = arch == 1
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return d, nil
}
func (r *SQLite) SaveDefinition(ctx context.Context, d Definition, create bool) error {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if create {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM attribute_definitions WHERE entity_type=? AND archived=0`, d.EntityType).Scan(&n); e != nil {
			return e
		}
		if n >= 50 {
			return errors.New("maximum 50 active definitions per entity type")
		}
		d.ID = uuid.NewString()
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
		d.UpdatedAt = d.CreatedAt
		_, e = tx.ExecContext(ctx, `INSERT INTO attribute_definitions(id,entity_type,key,label,value_type,required,min_number,max_number,max_length,archived,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,0,?,?)`, d.ID, d.EntityType, d.Key, d.Label, d.ValueType, boolInt(d.Required), d.MinNumber, d.MaxNumber, d.MaxLength, now, now)
	} else {
		var old Definition
		old, e = scanDefinition(tx.QueryRowContext(ctx, `SELECT id,entity_type,key,label,value_type,required,min_number,max_number,max_length,archived,created_at,updated_at FROM attribute_definitions WHERE id=?`, d.ID))
		if errors.Is(e, sql.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM entity_attribute_values WHERE definition_id=?`, d.ID).Scan(&count); e != nil {
			return e
		}
		if count > 0 && (old.Key != d.Key || old.ValueType != d.ValueType) {
			return errors.New("key and value type are immutable after values exist")
		}
		if count > 0 {
			rows, err := tx.QueryContext(ctx, `SELECT CASE ? WHEN 'string' THEN text_value ELSE NULL END,number_value FROM entity_attribute_values WHERE definition_id=?`, d.ValueType, d.ID)
			if err != nil {
				return err
			}
			for rows.Next() {
				var text sql.NullString
				var num sql.NullFloat64
				if err = rows.Scan(&text, &num); err != nil {
					rows.Close()
					return err
				}
				if text.Valid && d.MaxLength != nil && len(text.String) > *d.MaxLength {
					rows.Close()
					return errors.New("new definition bounds invalidate existing values")
				}
				if num.Valid && ((d.MinNumber != nil && num.Float64 < *d.MinNumber) || (d.MaxNumber != nil && num.Float64 > *d.MaxNumber)) {
					rows.Close()
					return errors.New("new definition bounds invalidate existing values")
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		d.EntityType = old.EntityType
		d.Key = old.Key
		d.ValueType = old.ValueType
		d.CreatedAt = old.CreatedAt
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, now)
		_, e = tx.ExecContext(ctx, `UPDATE attribute_definitions SET label=?,required=?,min_number=?,max_number=?,max_length=?,archived=?,updated_at=? WHERE id=?`, d.Label, boolInt(d.Required), d.MinNumber, d.MaxNumber, d.MaxLength, boolInt(d.Archived), now, d.ID)
	}
	if e != nil {
		return e
	}
	action := "attribute_definition.updated"
	if create {
		action = "attribute_definition.created"
	}
	if e = auth.AuditTx(ctx, tx, action, "attribute_definition", d.ID, auth.RequestID(ctx), map[string]any{"entity_type": d.EntityType}); e != nil {
		return e
	}
	return tx.Commit()
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *SQLite) AttributeValues(ctx context.Context, id string) (string, map[string]any, map[string]any, error) {
	var typ string
	e := r.db.QueryRowContext(ctx, `SELECT type FROM entities WHERE id=?`, id).Scan(&typ)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil, nil, ErrNotFound
	}
	if e != nil {
		return "", nil, nil, e
	}
	current, previous := map[string]any{}, map[string]any{}
	rows, e := r.db.QueryContext(ctx, `SELECT d.entity_type,d.key,d.value_type,v.text_value,v.number_value,v.boolean_value FROM entity_attribute_values v JOIN attribute_definitions d ON d.id=v.definition_id WHERE v.entity_id=? ORDER BY d.key`, id)
	if e != nil {
		return "", nil, nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var dt, key, vt string
		var text sql.NullString
		var num sql.NullFloat64
		var boolean sql.NullInt64
		if e = rows.Scan(&dt, &key, &vt, &text, &num, &boolean); e != nil {
			return "", nil, nil, e
		}
		var value any
		switch vt {
		case "string":
			value = text.String
		case "number":
			value = num.Float64
		case "boolean":
			value = boolean.Int64 == 1
		}
		if dt == typ {
			current[key] = value
		} else {
			previous[dt+"."+key] = value
		}
	}
	return typ, current, previous, rows.Err()
}

func (r *SQLite) ReplaceAttributes(ctx context.Context, id string, values map[string]any) error {
	encoded, e := json.Marshal(values)
	if e != nil {
		return e
	}
	if len(encoded) > 32<<10 {
		return errors.New("attributes exceed 32 KiB")
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var typ string
	if e = tx.QueryRowContext(ctx, `SELECT type FROM entities WHERE id=?`, id).Scan(&typ); errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,key,value_type,required,min_number,max_number,max_length FROM attribute_definitions WHERE entity_type=? AND archived=0`, typ)
	if e != nil {
		return e
	}
	defs := map[string]Definition{}
	for rows.Next() {
		var d Definition
		var req int
		if e = rows.Scan(&d.ID, &d.Key, &d.ValueType, &req, &d.MinNumber, &d.MaxNumber, &d.MaxLength); e != nil {
			rows.Close()
			return e
		}
		d.Required = req == 1
		defs[d.Key] = d
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for key, value := range values {
		d, ok := defs[key]
		if !ok {
			return fmt.Errorf("unknown or archived attribute %q", key)
		}
		switch d.ValueType {
		case "string":
			s, ok := value.(string)
			if !ok {
				return fmt.Errorf("attribute %s must be a string", key)
			}
			max := 2048
			if d.MaxLength != nil {
				max = *d.MaxLength
			}
			if len(s) > max {
				return fmt.Errorf("attribute %s exceeds maximum length", key)
			}
		case "number":
			n, ok := value.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("attribute %s must be a finite number", key)
			}
			if d.MinNumber != nil && n < *d.MinNumber || d.MaxNumber != nil && n > *d.MaxNumber {
				return fmt.Errorf("attribute %s is outside its numeric bounds", key)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("attribute %s must be a boolean", key)
			}
		}
	}
	for key, d := range defs {
		if d.Required {
			if _, ok := values[key]; !ok {
				return fmt.Errorf("required attribute %s is missing", key)
			}
		}
	}
	before := map[string]any{}
	oldRows, e := tx.QueryContext(ctx, `SELECT d.key,d.value_type,v.text_value,v.number_value,v.boolean_value FROM entity_attribute_values v JOIN attribute_definitions d ON d.id=v.definition_id WHERE v.entity_id=? AND d.entity_type=? AND d.archived=0`, id, typ)
	if e != nil {
		return e
	}
	for oldRows.Next() {
		var key, kind string
		var text sql.NullString
		var number sql.NullFloat64
		var boolean sql.NullInt64
		if e = oldRows.Scan(&key, &kind, &text, &number, &boolean); e != nil {
			oldRows.Close()
			return e
		}
		switch kind {
		case "string":
			before[key] = text.String
		case "number":
			before[key] = number.Float64
		case "boolean":
			before[key] = boolean.Int64 == 1
		}
	}
	e = oldRows.Err()
	oldRows.Close()
	if e != nil {
		return e
	}
	if reflect.DeepEqual(before, values) {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM entity_attribute_values WHERE entity_id=? AND definition_id IN (SELECT id FROM attribute_definitions WHERE entity_type=? AND archived=0)`, id, typ); e != nil {
		return e
	}
	for key, value := range values {
		d := defs[key]
		var text any
		var number any
		var boolean any
		switch d.ValueType {
		case "string":
			text = value
		case "number":
			number = value
		case "boolean":
			if value.(bool) {
				boolean = 1
			} else {
				boolean = 0
			}
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO entity_attribute_values(entity_id,definition_id,text_value,number_value,boolean_value) VALUES(?,?,?,?,?)`, id, d.ID, text, number, boolean); e != nil {
			return e
		}
	}
	if e = insertActivity(ctx, tx, id, "entity.attributes.updated", map[string]any{"before": activityAttributeValues(before), "after": activityAttributeValues(values)}); e != nil {
		return e
	}
	if e = auth.AuditTx(ctx, tx, "entity.attributes.updated", "entity", id, auth.RequestID(ctx), map[string]any{"attribute_count": len(values)}); e != nil {
		return e
	}
	return tx.Commit()
}

func normalizeName(s string, max int) (string, string, error) {
	display := strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if display == "" || len([]rune(display)) > max {
		return "", "", errors.New("name is required or exceeds maximum length")
	}
	return display, strings.ToLower(display), nil
}
func (r *SQLite) Catalog(ctx context.Context, kind string) ([]CatalogItem, error) {
	table := catalogTable(kind)
	rows, e := r.db.QueryContext(ctx, `SELECT id,name FROM `+table+` ORDER BY normalized_name,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []CatalogItem{}
	for rows.Next() {
		var v CatalogItem
		if e = rows.Scan(&v.ID, &v.Name); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func catalogTable(k string) string {
	if k == "categories" {
		return "categories"
	}
	return "tags"
}
func (r *SQLite) CreateCatalog(ctx context.Context, kind, name string) (CatalogItem, error) {
	table := catalogTable(kind)
	limit := 64
	if table == "categories" {
		limit = 100
	}
	display, norm, e := normalizeName(name, limit)
	if e != nil {
		return CatalogItem{}, e
	}
	item := CatalogItem{ID: uuid.NewString(), Name: display}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return CatalogItem{}, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO `+table+`(id,name,normalized_name,created_at) VALUES(?,?,?,?)`, item.ID, item.Name, norm, time.Now().UTC().Format(time.RFC3339Nano))
	if e == nil {
		e = auth.AuditTx(ctx, tx, table+".created", table, item.ID, auth.RequestID(ctx), map[string]any{})
	}
	if e == nil {
		e = tx.Commit()
	}
	return item, e
}
func (r *SQLite) DeleteCatalog(ctx context.Context, kind, id string) error {
	table := catalogTable(kind)
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e == nil && n == 0 {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if e = auth.AuditTx(ctx, tx, table+".deleted", table, id, auth.RequestID(ctx), map[string]any{}); e != nil {
		return e
	}
	return tx.Commit()
}
func (r *SQLite) EntityCatalog(ctx context.Context, id, kind string) ([]CatalogItem, error) {
	if _, e := r.Get(ctx, id); e != nil {
		return nil, e
	}
	table := catalogTable(kind)
	link := "entity_tags"
	fk := "tag_id"
	if table == "categories" {
		link = "entity_categories"
		fk = "category_id"
	}
	rows, e := r.db.QueryContext(ctx, `SELECT c.id,c.name FROM `+link+` x JOIN `+table+` c ON c.id=x.`+fk+` WHERE x.entity_id=? ORDER BY c.normalized_name,c.id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []CatalogItem{}
	for rows.Next() {
		var x CatalogItem
		if e = rows.Scan(&x.ID, &x.Name); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *SQLite) ReplaceCatalog(ctx context.Context, id, kind string, ids []string) error {
	limit := 50
	table, link, fk, event := "tags", "entity_tags", "tag_id", "entity.tags.updated"
	if kind == "categories" {
		limit = 20
		table, link, fk, event = "categories", "entity_categories", "category_id", "entity.categories.updated"
	}
	set := map[string]bool{}
	unique := []string{}
	for _, v := range ids {
		if !set[v] {
			set[v] = true
			unique = append(unique, v)
		}
	}
	if len(unique) > limit {
		return errors.New("assignment limit exceeded")
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM entities WHERE id=?`, id).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		return ErrNotFound
	}
	for _, v := range unique {
		if _, e = uuid.Parse(v); e != nil {
			return errors.New("assignment IDs must be UUIDs")
		}
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE id=?`, v).Scan(&n); e != nil {
			return e
		}
		if n == 0 {
			return errors.New("unknown catalog ID")
		}
	}
	oldRows, e := tx.QueryContext(ctx, `SELECT `+fk+` FROM `+link+` WHERE entity_id=? ORDER BY `+fk, id)
	if e != nil {
		return e
	}
	old := []string{}
	for oldRows.Next() {
		var v string
		if e = oldRows.Scan(&v); e != nil {
			oldRows.Close()
			return e
		}
		old = append(old, v)
	}
	e = oldRows.Err()
	oldRows.Close()
	if e != nil {
		return e
	}
	if sameStrings(old, unique) {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM `+link+` WHERE entity_id=?`, id); e != nil {
		return e
	}
	for _, v := range unique {
		if _, e = tx.ExecContext(ctx, `INSERT INTO `+link+`(entity_id,`+fk+`) VALUES(?,?)`, id, v); e != nil {
			return e
		}
	}
	beforeItems := []CatalogItem{}
	for _, v := range old {
		var x CatalogItem
		if e = tx.QueryRowContext(ctx, `SELECT id,name FROM `+table+` WHERE id=?`, v).Scan(&x.ID, &x.Name); e != nil {
			return e
		}
		beforeItems = append(beforeItems, x)
	}
	items := []CatalogItem{}
	for _, v := range unique {
		var x CatalogItem
		if e = tx.QueryRowContext(ctx, `SELECT id,name FROM `+table+` WHERE id=?`, v).Scan(&x.ID, &x.Name); e != nil {
			return e
		}
		items = append(items, x)
	}
	if e = insertActivity(ctx, tx, id, event, map[string]any{"before": beforeItems, "after": items}); e != nil {
		return e
	}
	if e = auth.AuditTx(ctx, tx, event, "entity", id, auth.RequestID(ctx), map[string]any{"assignment_count": len(items)}); e != nil {
		return e
	}
	return tx.Commit()
}
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string{}, a...)
	bb := append([]string{}, b...)
	sortStrings(aa)
	sortStrings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
func sortStrings(x []string) {
	for i := 1; i < len(x); i++ {
		for j := i; j > 0 && x[j] < x[j-1]; j-- {
			x[j], x[j-1] = x[j-1], x[j]
		}
	}
}

func (r *SQLite) Activities(ctx context.Context, entityID string, limit int, token string) (ActivityPage, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return ActivityPage{}, errors.New("limit must be between 1 and 100")
	}
	var c cursor
	if token != "" {
		b, e := base64.RawURLEncoding.DecodeString(token)
		if e != nil || json.Unmarshal(b, &c) != nil || c.Time == "" || c.ID == "" {
			return ActivityPage{}, errors.New("invalid cursor")
		}
	}
	query := `SELECT id,entity_id,event_type,occurred_at,actor_kind,actor_id,payload_version,payload FROM activity_events WHERE entity_id=?`
	args := []any{entityID}
	if token != "" {
		query += ` AND (occurred_at < ? OR (occurred_at = ? AND id < ?))`
		args = append(args, c.Time, c.Time, c.ID)
	}
	query += ` ORDER BY occurred_at DESC,id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, e := r.db.QueryContext(ctx, query, args...)
	if e != nil {
		return ActivityPage{}, e
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		var ts string
		var actor sql.NullString
		var payload string
		if e = rows.Scan(&a.ID, &a.EntityID, &a.EventType, &ts, &a.Actor.Kind, &actor, &a.PayloadVersion, &payload); e != nil {
			return ActivityPage{}, e
		}
		a.OccurredAt, _ = time.Parse(time.RFC3339Nano, ts)
		if actor.Valid {
			a.Actor.ID = &actor.String
		}
		a.Changes = json.RawMessage(payload)
		out = append(out, a)
	}
	if e = rows.Err(); e != nil {
		return ActivityPage{}, e
	}
	next := ""
	if len(out) > limit {
		last := out[limit-1]
		b, _ := json.Marshal(cursor{Time: last.OccurredAt.UTC().Format(activityTimestampFormat), ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(b)
		out = out[:limit]
	}
	return ActivityPage{Data: out, NextCursor: next}, nil
}
