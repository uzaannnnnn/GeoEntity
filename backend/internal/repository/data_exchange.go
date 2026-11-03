package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"geoentity/backend/internal/entity"

	"github.com/google/uuid"
	"modernc.org/sqlite"
)

var (
	ErrImportConflict    = errors.New("import conflicts with current database state")
	ErrImportInvalid     = errors.New("import metadata is invalid")
	ErrExportResultLimit = errors.New("export result limit exceeded")
)

type ExportEntity struct {
	Entity
	Attributes         map[string]any `json:"attributes"`
	PreviousTypeValues map[string]any `json:"previous_type_values"`
	Tags               []string       `json:"tags"`
	Categories         []string       `json:"categories"`
}

// Entity is embedded to keep export DTO construction in the repository explicit.
type Entity = entity.Entity

type ImportRecord struct {
	Entity             entity.Entity
	Attributes         map[string]any
	PreviousTypeValues map[string]any
	Tags               []string
	Categories         []string
	SourceID           string
	SourceIndex        int
}

func (r *SQLite) CountExportEntities(ctx context.Context, filters EntityFilters, max int) (int, error) {
	filters, err := NormalizeEntityFilters(filters)
	if err != nil {
		return 0, err
	}
	where, args := entityWhere("e", filters, true)
	var count int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entities e`+where, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count > max {
		return count, ErrExportResultLimit
	}
	return count, nil
}

type ImportRowIssue struct {
	Index   int
	Field   string
	Code    string
	Message string
}

type ImportPreflight struct {
	Issues          []ImportRowIssue
	ExistingIDs     []string
	DuplicateNames  map[int][]string
	NewCatalogItems map[string][]string
}

func (r *SQLite) ExportEntityBatch(ctx context.Context, filters EntityFilters, afterCreated, afterID string, limit int) ([]ExportEntity, error) {
	filters, err := NormalizeEntityFilters(filters)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidQuery
	}
	where, args := entityWhere("e", filters, true)
	if afterCreated != "" {
		where += ` AND (e.created_at > ? OR (e.created_at = ? AND e.id > ?))`
		args = append(args, afterCreated, afterCreated, afterID)
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.name,e.type,e.description,e.status,e.latitude,e.longitude,e.created_at,e.updated_at FROM entities e`+where+` ORDER BY e.created_at ASC,e.id ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	items := make([]ExportEntity, 0, limit)
	ids := make([]string, 0, limit)
	types := make(map[string]string, limit)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		items = append(items, ExportEntity{Entity: item, Attributes: map[string]any{}, PreviousTypeValues: map[string]any{}, Tags: []string{}, Categories: []string{}})
		ids = append(ids, item.ID)
		types[item.ID] = item.Type
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	placeholders, idArgs := sqlIn(ids)
	attrRows, err := r.db.QueryContext(ctx, `SELECT v.entity_id,d.entity_type,d.key,d.value_type,v.text_value,v.number_value,v.boolean_value FROM entity_attribute_values v JOIN attribute_definitions d ON d.id=v.definition_id WHERE v.entity_id IN (`+placeholders+`) ORDER BY v.entity_id,d.entity_type,d.key`, idArgs...)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*ExportEntity, len(items))
	for i := range items {
		byID[items[i].ID] = &items[i]
	}
	for attrRows.Next() {
		var id, typ, key, kind string
		var text sql.NullString
		var number sql.NullFloat64
		var boolean sql.NullInt64
		if err := attrRows.Scan(&id, &typ, &key, &kind, &text, &number, &boolean); err != nil {
			attrRows.Close()
			return nil, err
		}
		var value any
		switch kind {
		case "string":
			value = text.String
		case "number":
			value = number.Float64
		case "boolean":
			value = boolean.Int64 == 1
		}
		if types[id] == typ {
			byID[id].Attributes[key] = value
		} else {
			byID[id].PreviousTypeValues[typ+"."+key] = value
		}
	}
	if err := attrRows.Err(); err != nil {
		attrRows.Close()
		return nil, err
	}
	attrRows.Close()
	for _, spec := range []struct{ table, link, fk, out string }{
		{"tags", "entity_tags", "tag_id", "tags"}, {"categories", "entity_categories", "category_id", "categories"},
	} {
		query := `SELECT x.entity_id,c.name FROM ` + spec.link + ` x JOIN ` + spec.table + ` c ON c.id=x.` + spec.fk + ` WHERE x.entity_id IN (` + placeholders + `) ORDER BY x.entity_id,c.normalized_name,c.id`
		catalogRows, err := r.db.QueryContext(ctx, query, idArgs...)
		if err != nil {
			return nil, err
		}
		for catalogRows.Next() {
			var id, name string
			if err := catalogRows.Scan(&id, &name); err != nil {
				catalogRows.Close()
				return nil, err
			}
			if spec.out == "tags" {
				byID[id].Tags = append(byID[id].Tags, name)
			} else {
				byID[id].Categories = append(byID[id].Categories, name)
			}
		}
		if err := catalogRows.Err(); err != nil {
			catalogRows.Close()
			return nil, err
		}
		catalogRows.Close()
	}
	return items, nil
}

func sqlIn(values []string) (string, []any) {
	parts := make([]string, len(values))
	args := make([]any, len(values))
	for i, value := range values {
		parts[i] = "?"
		args[i] = value
	}
	return strings.Join(parts, ","), args
}

func (r *SQLite) PreflightImport(ctx context.Context, records []ImportRecord) (ImportPreflight, error) {
	result := ImportPreflight{Issues: []ImportRowIssue{}, ExistingIDs: []string{}, DuplicateNames: map[int][]string{}, NewCatalogItems: map[string][]string{}}
	seenIDs, seenNames := map[string]int{}, map[string]int{}
	newCatalog := map[string]map[string]string{"tag": {}, "category": {}}
	for i, record := range records {
		index := record.SourceIndex
		if index == 0 {
			index = i + 1
		}
		if prior, ok := seenIDs[record.Entity.ID]; ok {
			result.Issues = append(result.Issues, ImportRowIssue{index, "id", "DUPLICATE_ID", fmt.Sprintf("ID duplicates record %d", prior+1)})
		} else {
			seenIDs[record.Entity.ID] = index
		}
		var exists int
		if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM entities WHERE id=?`, record.Entity.ID).Scan(&exists); err != nil {
			return result, err
		}
		if exists > 0 {
			result.ExistingIDs = append(result.ExistingIDs, record.Entity.ID)
			result.Issues = append(result.Issues, ImportRowIssue{index, "id", "ID_EXISTS", "Entity ID already exists"})
		}
		display, norm, _ := normalizeName(record.Entity.Name, 2048)
		if norm != "" {
			var existing []string
			rows, err := r.db.QueryContext(ctx, `SELECT id FROM entities WHERE lower(trim(name))=lower(trim(?)) ORDER BY id LIMIT 10`, display)
			if err != nil {
				return result, err
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return result, err
				}
				existing = append(existing, id)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return result, err
			}
			rows.Close()
			if len(existing) > 0 {
				result.DuplicateNames[index] = existing
			}
			if prior, ok := seenNames[norm]; ok {
				result.DuplicateNames[index] = append(result.DuplicateNames[index], fmt.Sprintf("row:%d", prior))
			} else {
				seenNames[norm] = index
			}
		}
		if err := r.validateImportMetadata(ctx, record); err != nil {
			result.Issues = append(result.Issues, ImportRowIssue{index, "attributes", "INVALID_METADATA", err.Error()})
		}
		for _, kindNames := range []struct {
			kind  string
			names []string
		}{{"tag", record.Tags}, {"category", record.Categories}} {
			table, limit := "tags", 64
			if kindNames.kind == "category" {
				table, limit = "categories", 100
			}
			for _, name := range kindNames.names {
				display, norm, err := normalizeName(name, limit)
				if err != nil {
					result.Issues = append(result.Issues, ImportRowIssue{index, kindNames.kind + "s", "INVALID_CATALOG_NAME", err.Error()})
					continue
				}
				var count int
				if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE normalized_name=?`, norm).Scan(&count); err != nil {
					return result, err
				}
				if count == 0 {
					newCatalog[kindNames.kind][norm] = display
				}
			}
		}
	}
	for kind, names := range newCatalog {
		for _, name := range names {
			result.NewCatalogItems[kind] = append(result.NewCatalogItems[kind], name)
		}
		sortStrings(result.NewCatalogItems[kind])
	}
	return result, nil
}

func (r *SQLite) validateImportMetadata(ctx context.Context, record ImportRecord) error {
	if len(record.Tags) > 50 || len(record.Categories) > 20 {
		return errors.New("tag or category assignment limit exceeded")
	}
	definitions, err := r.Definitions(ctx, record.Entity.Type)
	if err != nil {
		return err
	}
	defs := map[string]Definition{}
	for _, definition := range definitions {
		defs[definition.Key] = definition
	}
	for _, key := range sortedMapKeys(record.Attributes) {
		value := record.Attributes[key]
		d, ok := defs[key]
		if !ok || d.Archived {
			return fmt.Errorf("unknown or archived attribute %q", key)
		}
		if err := validateTypedAttribute(d, value); err != nil {
			return err
		}
	}
	for key, d := range defs {
		if d.Required && !d.Archived {
			if _, ok := record.Attributes[key]; !ok {
				return fmt.Errorf("required attribute %q is missing", key)
			}
		}
	}
	for _, key := range sortedMapKeys(record.PreviousTypeValues) {
		value := record.PreviousTypeValues[key]
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 || !validEntityType(parts[0]) {
			return fmt.Errorf("invalid previous attribute key %q", key)
		}
		all, err := r.Definitions(ctx, parts[0])
		if err != nil {
			return err
		}
		var found *Definition
		for i := range all {
			if all[i].Key == parts[1] {
				found = &all[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("unknown previous attribute %q", key)
		}
		if err := validateTypedAttribute(*found, value); err != nil {
			return err
		}
	}
	return nil
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func validateTypedAttribute(d Definition, value any) error {
	switch d.ValueType {
	case "string":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("attribute %s must be string", d.Key)
		}
		max := 2048
		if d.MaxLength != nil {
			max = *d.MaxLength
		}
		if len(v) > max {
			return fmt.Errorf("attribute %s exceeds maximum length", d.Key)
		}
	case "number":
		v, ok := value.(float64)
		if !ok || !finite(v) {
			return fmt.Errorf("attribute %s must be finite number", d.Key)
		}
		if d.MinNumber != nil && v < *d.MinNumber || d.MaxNumber != nil && v > *d.MaxNumber {
			return fmt.Errorf("attribute %s is outside bounds", d.Key)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("attribute %s must be boolean", d.Key)
		}
	default:
		return ErrImportInvalid
	}
	return nil
}

func (r *SQLite) CreateImportedEntities(ctx context.Context, format, batchID string, records []ImportRecord) (int, int, error) {
	if len(records) == 0 {
		return 0, 0, ErrImportInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	// Recheck IDs, definitions and catalogs after acquiring the write transaction.
	for _, record := range records {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM entities WHERE id=?`, record.Entity.ID).Scan(&count); err != nil {
			return 0, 0, err
		}
		if count > 0 {
			return 0, 0, ErrImportConflict
		}
	}
	createdTags, createdCategories := 0, 0
	for _, record := range records {
		if err := validateImportMetadataTx(ctx, tx, record); err != nil {
			return 0, 0, fmt.Errorf("%w: metadata changed or became invalid", ErrImportInvalid)
		}
		item := record.Entity
		if _, err := tx.ExecContext(ctx, `INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.Name, item.Type, item.Description, item.Status, item.Latitude, item.Longitude, item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			var sqliteErr *sqlite.Error
			if errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 19 {
				return 0, 0, ErrImportConflict
			}
			return 0, 0, err
		}
		allDefs, err := loadDefinitionMapTx(ctx, tx, item.Type, false)
		if err != nil {
			return 0, 0, err
		}
		for key, value := range record.Attributes {
			d := allDefs[key]
			if err := insertAttributeValue(ctx, tx, item.ID, d, value); err != nil {
				return 0, 0, err
			}
		}
		for key, value := range record.PreviousTypeValues {
			parts := strings.SplitN(key, ".", 2)
			defs, err := loadDefinitionMapTx(ctx, tx, parts[0], true)
			if err != nil {
				return 0, 0, err
			}
			d, ok := defs[parts[1]]
			if !ok {
				return 0, 0, ErrImportConflict
			}
			if err := insertAttributeValue(ctx, tx, item.ID, d, value); err != nil {
				return 0, 0, err
			}
		}
		for _, itemSpec := range []struct {
			kind, table, link, fk string
			names                 []string
		}{{"tag", "tags", "entity_tags", "tag_id", record.Tags}, {"category", "categories", "entity_categories", "category_id", record.Categories}} {
			limit := 64
			if itemSpec.kind == "category" {
				limit = 100
			}
			seen := map[string]bool{}
			for _, raw := range itemSpec.names {
				name, norm, err := normalizeName(raw, limit)
				if err != nil {
					return 0, 0, err
				}
				if seen[norm] {
					continue
				}
				seen[norm] = true
				id, err := resolveOrCreateCatalogTx(ctx, tx, itemSpec.table, name, norm)
				if err != nil {
					return 0, 0, err
				}
				if strings.HasPrefix(id, "created:") {
					id = strings.TrimPrefix(id, "created:")
					if itemSpec.kind == "tag" {
						createdTags++
					} else {
						createdCategories++
					}
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO `+itemSpec.link+`(entity_id,`+itemSpec.fk+`) VALUES(?,?)`, item.ID, id); err != nil {
					return 0, 0, err
				}
			}
		}
		payload := map[string]any{"changes": map[string]any{"source": "import", "format": format, "batch_id": batchID, "entity": activityEntity(item)}}
		if err := insertActivity(ctx, tx, item.ID, "entity.created", payload); err != nil {
			return 0, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return createdTags, createdCategories, nil
}

func validateImportMetadataTx(ctx context.Context, tx *sql.Tx, record ImportRecord) error {
	if len(record.Tags) > 50 || len(record.Categories) > 20 {
		return errors.New("tag or category assignment limit exceeded")
	}
	defs, err := loadDefinitionMapTx(ctx, tx, record.Entity.Type, false)
	if err != nil {
		return err
	}
	for key, value := range record.Attributes {
		d, ok := defs[key]
		if !ok {
			return fmt.Errorf("unknown or archived attribute %q", key)
		}
		if err := validateTypedAttribute(d, value); err != nil {
			return err
		}
	}
	for key, d := range defs {
		if d.Required {
			if _, ok := record.Attributes[key]; !ok {
				return fmt.Errorf("required attribute %q is missing", key)
			}
		}
	}
	for key, value := range record.PreviousTypeValues {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			return ErrImportInvalid
		}
		previous, err := loadDefinitionMapTx(ctx, tx, parts[0], true)
		if err != nil {
			return err
		}
		d, ok := previous[parts[1]]
		if !ok {
			return fmt.Errorf("unknown previous attribute %q", key)
		}
		if err := validateTypedAttribute(d, value); err != nil {
			return err
		}
	}
	return nil
}

func loadDefinitionMapTx(ctx context.Context, tx *sql.Tx, typ string, includeArchived bool) (map[string]Definition, error) {
	query := `SELECT id,entity_type,key,label,value_type,required,min_number,max_number,max_length,archived,created_at,updated_at FROM attribute_definitions WHERE entity_type=?`
	if !includeArchived {
		query += ` AND archived=0`
	}
	rows, err := tx.QueryContext(ctx, query, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Definition{}
	for rows.Next() {
		d, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		out[d.Key] = d
	}
	return out, rows.Err()
}

func insertAttributeValue(ctx context.Context, tx *sql.Tx, entityID string, d Definition, value any) error {
	var text, number, boolean any
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
	default:
		return ErrImportInvalid
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO entity_attribute_values(entity_id,definition_id,text_value,number_value,boolean_value) VALUES(?,?,?,?,?)`, entityID, d.ID, text, number, boolean)
	return err
}

func resolveOrCreateCatalogTx(ctx context.Context, tx *sql.Tx, table, name, norm string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM `+table+` WHERE normalized_name=?`, norm).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+`(id,name,normalized_name,created_at) VALUES(?,?,?,?)`, id, name, norm, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return "", err
	}
	return "created:" + id, nil
}

func JSONValue(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
