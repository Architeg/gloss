package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// EntryRepo persists glossary entries.
type EntryRepo struct {
	db *sql.DB
}

// NewEntryRepo wraps a database handle.
func NewEntryRepo(db *sql.DB) *EntryRepo {
	return &EntryRepo{db: db}
}

// CreateEntry inserts a new row. Command is normalized; timestamps default to UTC now.
func (r *EntryRepo) CreateEntry(ctx context.Context, e model.Entry) (int64, error) {
	cmd := model.NormalizeCommand(e.Command)
	if cmd == "" {
		return 0, fmt.Errorf("command is required")
	}
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = now
	}
	tags, err := json.Marshal(e.Tags)
	if err != nil {
		return 0, fmt.Errorf("encode tags: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO entries (command, description, tags, type, source, target, managed_alias, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cmd,
		e.Description,
		string(tags),
		e.Type,
		e.Source,
		e.Target,
		boolToInt(e.ManagedAlias),
		e.CreatedAt.Format(time.RFC3339Nano),
		e.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("insert entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetAllEntries returns every entry sorted by command.
func (r *EntryRepo) GetAllEntries(ctx context.Context) ([]model.Entry, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, command, description, tags, type, source, target, managed_alias, created_at, updated_at
FROM entries
ORDER BY command COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var out []model.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries: %w", err)
	}
	return out, nil
}

// GetEntriesByTag filters entries that contain tag (exact match on tag string).
func (r *EntryRepo) GetEntriesByTag(ctx context.Context, tag string) ([]model.Entry, error) {
	all, err := r.GetAllEntries(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Entry
	for _, e := range all {
		for _, t := range e.Tags {
			if t == tag {
				out = append(out, e)
				break
			}
		}
	}
	return out, nil
}

// GetEntryByCommand loads one entry by normalized command.
func (r *EntryRepo) GetEntryByCommand(ctx context.Context, command string) (model.Entry, error) {
	cmd := model.NormalizeCommand(command)
	row := r.db.QueryRowContext(ctx, `
SELECT id, command, description, tags, type, source, target, managed_alias, created_at, updated_at
FROM entries WHERE command = ?`, cmd)
	e, err := scanEntryRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Entry{}, err
		}
		return model.Entry{}, err
	}
	return e, nil
}

// UpdateEntry replaces fields for an existing id and bumps updated_at.
func (r *EntryRepo) UpdateEntry(ctx context.Context, e model.Entry) error {
	if e.ID == 0 {
		return fmt.Errorf("entry id is required")
	}
	cmd := model.NormalizeCommand(e.Command)
	if cmd == "" {
		return fmt.Errorf("command is required")
	}
	tags, err := json.Marshal(e.Tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	now := time.Now().UTC()
	if !e.UpdatedAt.IsZero() {
		now = e.UpdatedAt.UTC()
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE entries
SET command = ?, description = ?, tags = ?, type = ?, source = ?, target = ?, managed_alias = ?, updated_at = ?
WHERE id = ?`,
		cmd,
		e.Description,
		string(tags),
		e.Type,
		e.Source,
		e.Target,
		boolToInt(e.ManagedAlias),
		now.Format(time.RFC3339Nano),
		e.ID,
	)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteEntryByCommand removes a row by normalized command.
func (r *EntryRepo) DeleteEntryByCommand(ctx context.Context, command string) error {
	cmd := model.NormalizeCommand(command)
	res, err := r.db.ExecContext(ctx, `DELETE FROM entries WHERE command = ?`, cmd)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanEntry(rows *sql.Rows) (model.Entry, error) {
	var (
		id                                                  int64
		command, description, tagsJSON, typ, source, target string
		managed                                             int
		createdAtStr, updatedAtStr                          string
	)
	if err := rows.Scan(&id, &command, &description, &tagsJSON, &typ, &source, &target, &managed, &createdAtStr, &updatedAtStr); err != nil {
		return model.Entry{}, fmt.Errorf("scan entry: %w", err)
	}
	return buildEntry(id, command, description, tagsJSON, typ, source, target, managed, createdAtStr, updatedAtStr)
}

func scanEntryRow(row *sql.Row) (model.Entry, error) {
	var (
		id                                                  int64
		command, description, tagsJSON, typ, source, target string
		managed                                             int
		createdAtStr, updatedAtStr                          string
	)
	if err := row.Scan(&id, &command, &description, &tagsJSON, &typ, &source, &target, &managed, &createdAtStr, &updatedAtStr); err != nil {
		return model.Entry{}, fmt.Errorf("scan entry: %w", err)
	}
	return buildEntry(id, command, description, tagsJSON, typ, source, target, managed, createdAtStr, updatedAtStr)
}

func buildEntry(id int64, command, description, tagsJSON, typ, source, target string, managed int, createdAtStr, updatedAtStr string) (model.Entry, error) {
	var tags []string
	if tagsJSON != "" && tagsJSON != "null" {
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			return model.Entry{}, fmt.Errorf("decode tags: %w", err)
		}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		createdAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return model.Entry{}, fmt.Errorf("parse created_at: %w", err)
		}
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtStr)
	if err != nil {
		updatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return model.Entry{}, fmt.Errorf("parse updated_at: %w", err)
		}
	}
	return model.Entry{
		ID:           id,
		Command:      command,
		Description:  description,
		Tags:         tags,
		Type:         typ,
		Source:       source,
		Target:       target,
		ManagedAlias: managed != 0,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}
