package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const timeLayout = time.RFC3339Nano

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func fmtTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func fmtTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return fmtTime(*t)
}

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	parsed, err := time.Parse(timeLayout, s.String)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Store) ReplaceTools(ctx context.Context, tools []Tool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tools transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO tools (
	name, path, real_path, active, family, version, origin, managed_by,
	writable, updateable, risk, source, latest_version, latest_url,
	latest_published_at, latest_security, checked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name, real_path) DO UPDATE SET
	path = excluded.path,
	active = excluded.active,
	family = excluded.family,
	version = excluded.version,
	origin = excluded.origin,
	managed_by = excluded.managed_by,
	writable = excluded.writable,
	updateable = excluded.updateable,
	risk = excluded.risk,
	source = excluded.source,
	latest_version = excluded.latest_version,
	latest_url = excluded.latest_url,
	latest_published_at = excluded.latest_published_at,
	latest_security = excluded.latest_security,
	checked_at = excluded.checked_at`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare tools upsert: %w", err)
	}
	defer stmt.Close()

	keep := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		keep[tool.Name+"\x00"+tool.RealPath] = struct{}{}
		if _, err := stmt.ExecContext(ctx,
			tool.Name, tool.Path, tool.RealPath, boolInt(tool.Active), tool.Family,
			tool.Version, tool.Origin, tool.ManagedBy, boolInt(tool.Writable),
			boolInt(tool.Updateable), tool.Risk, tool.Source, tool.LatestVersion,
			tool.LatestURL, fmtTimePtr(tool.LatestPublishedAt), boolInt(tool.LatestSecurity),
			fmtTimePtr(tool.CheckedAt),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("upsert tool %s: %w", tool.Name, err)
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, name, real_path FROM tools`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("list stored tools: %w", err)
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var name, realPath string
		if err := rows.Scan(&id, &name, &realPath); err != nil {
			rows.Close()
			tx.Rollback()
			return fmt.Errorf("scan stored tool: %w", err)
		}
		if _, ok := keep[name+"\x00"+realPath]; !ok {
			stale = append(stale, id)
		}
	}
	if err := rows.Close(); err != nil {
		tx.Rollback()
		return fmt.Errorf("close stored tools: %w", err)
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM tools WHERE id = ?`, id); err != nil {
			tx.Rollback()
			return fmt.Errorf("delete stale tool %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tools: %w", err)
	}
	return nil
}

const toolColumns = `id, name, path, real_path, active, family, version, origin,
	managed_by, writable, updateable, risk, source, latest_version, latest_url,
	latest_published_at, latest_security, checked_at`

func scanTool(scanner interface{ Scan(...any) error }) (Tool, error) {
	var tool Tool
	var active, writable, updateable, security int
	var published, checked sql.NullString
	err := scanner.Scan(
		&tool.ID, &tool.Name, &tool.Path, &tool.RealPath, &active, &tool.Family,
		&tool.Version, &tool.Origin, &tool.ManagedBy, &writable, &updateable,
		&tool.Risk, &tool.Source, &tool.LatestVersion, &tool.LatestURL,
		&published, &security, &checked,
	)
	if err != nil {
		return Tool{}, err
	}
	tool.Active = active != 0
	tool.Writable = writable != 0
	tool.Updateable = updateable != 0
	tool.LatestSecurity = security != 0
	tool.LatestPublishedAt = parseTime(published)
	tool.CheckedAt = parseTime(checked)
	return tool, nil
}

func (s *Store) ListTools(ctx context.Context) ([]Tool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+toolColumns+` FROM tools ORDER BY active DESC, name, path`)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	defer rows.Close()

	var tools []Tool
	for rows.Next() {
		tool, err := scanTool(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tool: %w", err)
		}
		tools = append(tools, tool)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tools: %w", err)
	}
	return tools, nil
}

func (s *Store) GetTool(ctx context.Context, id int64) (Tool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+toolColumns+` FROM tools WHERE id = ?`, id)
	tool, err := scanTool(row)
	if err != nil {
		return Tool{}, fmt.Errorf("get tool %d: %w", id, err)
	}
	return tool, nil
}

func (s *Store) SetToolVersion(ctx context.Context, id int64, version string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE tools SET version = ? WHERE id = ?`, version, id); err != nil {
		return fmt.Errorf("set tool version: %w", err)
	}
	return nil
}

func (s *Store) RecordCheck(ctx context.Context, rec CheckRecord) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO checks (started_at, finished_at, status, tools, updates, error)
VALUES (?, ?, ?, ?, ?, ?)`,
		fmtTime(rec.StartedAt), fmtTime(rec.FinishedAt), rec.Status, rec.Tools, rec.Updates, rec.Error)
	if err != nil {
		return 0, fmt.Errorf("record check: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("check id: %w", err)
	}
	return id, nil
}

func (s *Store) ListChecks(ctx context.Context, limit int) ([]CheckRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, started_at, finished_at, status, tools, updates, error
FROM checks ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}
	defer rows.Close()

	var records []CheckRecord
	for rows.Next() {
		var rec CheckRecord
		var started, finished string
		if err := rows.Scan(&rec.ID, &started, &finished, &rec.Status, &rec.Tools, &rec.Updates, &rec.Error); err != nil {
			return nil, fmt.Errorf("scan check: %w", err)
		}
		rec.StartedAt, _ = time.Parse(timeLayout, started)
		rec.FinishedAt, _ = time.Parse(timeLayout, finished)
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *Store) RecordUpdate(ctx context.Context, rec UpdateRecord) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO tool_updates (tool_id, tool_name, from_version, to_version, command, status, output, started_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ToolID, rec.ToolName, rec.FromVersion, rec.ToVersion, rec.Command,
		rec.Status, rec.Output, fmtTime(rec.StartedAt), fmtTime(rec.FinishedAt))
	if err != nil {
		return 0, fmt.Errorf("record update: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("update id: %w", err)
	}
	return id, nil
}

func (s *Store) ListUpdates(ctx context.Context, toolID int64, limit int) ([]UpdateRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, tool_id, tool_name, from_version, to_version, command, status, output, started_at, finished_at
FROM tool_updates`
	args := []any{}
	if toolID > 0 {
		query += ` WHERE tool_id = ?`
		args = append(args, toolID)
	}
	query += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list updates: %w", err)
	}
	defer rows.Close()

	var records []UpdateRecord
	for rows.Next() {
		var rec UpdateRecord
		var started, finished string
		if err := rows.Scan(&rec.ID, &rec.ToolID, &rec.ToolName, &rec.FromVersion, &rec.ToVersion,
			&rec.Command, &rec.Status, &rec.Output, &started, &finished); err != nil {
			return nil, fmt.Errorf("scan update: %w", err)
		}
		rec.StartedAt, _ = time.Parse(timeLayout, started)
		rec.FinishedAt, _ = time.Parse(timeLayout, finished)
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *Store) WasNotified(ctx context.Context, toolID int64, latestVersion string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM tool_notifications WHERE tool_id = ? AND latest_version = ?`,
		toolID, latestVersion).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check notification: %w", err)
	}
	return true, nil
}

func (s *Store) RecordNotification(ctx context.Context, toolID int64, latestVersion string, sentAt time.Time) error {
	if _, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO tool_notifications (tool_id, latest_version, sent_at)
VALUES (?, ?, ?)`, toolID, latestVersion, fmtTime(sentAt)); err != nil {
		return fmt.Errorf("record notification: %w", err)
	}
	return nil
}
