package repository

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"geoentity/backend/internal/entity"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("entity not found")

//go:embed migrations/*.sql
var migrations embed.FS

type SQLite struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection keeps in-memory databases deterministic and SQLite
	// writes serialized for this small MVP service.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func (r *SQLite) Close() error { return r.db.Close() }

func Migrate(ctx context.Context, db *sql.DB) error {
	query, err := migrations.ReadFile("migrations/001_create_entities.sql")
	if err != nil {
		return fmt.Errorf("read sqlite migration: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, string(query))
	if err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration: %w", err)
	}
	return nil
}

func (r *SQLite) List(ctx context.Context) ([]entity.Entity, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, description, status, latitude, longitude, created_at, updated_at FROM entities ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]entity.Entity, 0)
	for rows.Next() {
		item, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLite) Get(ctx context.Context, id string) (entity.Entity, error) {
	item, err := scanEntity(r.db.QueryRowContext(ctx, `SELECT id, name, type, description, status, latitude, longitude, created_at, updated_at FROM entities WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Entity{}, ErrNotFound
	}
	return item, err
}

func (r *SQLite) Create(ctx context.Context, item entity.Entity) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO entities (id, name, type, description, status, latitude, longitude, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.Type, item.Description, item.Status, item.Latitude, item.Longitude, item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (r *SQLite) Update(ctx context.Context, item entity.Entity) error {
	result, err := r.db.ExecContext(ctx, `UPDATE entities SET name = ?, type = ?, description = ?, status = ?, latitude = ?, longitude = ?, updated_at = ? WHERE id = ?`,
		item.Name, item.Type, item.Description, item.Status, item.Latitude, item.Longitude, item.UpdatedAt.UTC().Format(time.RFC3339Nano), item.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLite) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM entities WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanEntity(row scanner) (entity.Entity, error) {
	var item entity.Entity
	var createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Status, &item.Latitude, &item.Longitude, &createdAt, &updatedAt); err != nil {
		return entity.Entity{}, err
	}
	var err error
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return entity.Entity{}, err
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return entity.Entity{}, err
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}
