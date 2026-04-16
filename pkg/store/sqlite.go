package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type sqliteStore struct {
	db      *sql.DB
	queue   chan Record
	closed  atomic.Bool
	wg      sync.WaitGroup
	workers int
}

func NewSQLiteStore(dsn string) (Store, error) {
	dsnWithParams := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL", dsn)
	db, err := sql.Open("sqlite3", dsnWithParams)
	if err != nil {
		return nil, err
	}

	// 限制连接池以减少 SQLite 锁竞争
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	s := &sqliteStore{
		db:      db,
		queue:   make(chan Record, 5000),
		workers: 3, // 生产建议 2-4 个 worker 对应磁盘 I/O
	}

	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	return s, nil
}

func (s *sqliteStore) initSchema() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS records (
		id TEXT PRIMARY KEY,
		trace_id TEXT,
		project TEXT,
		model TEXT,
		input_tokens INTEGER,
		output_tokens INTEGER,
		cost_millicents INTEGER,
		route_target TEXT,
		duration_ms INTEGER,
		timestamp TEXT, 
		date TEXT GENERATED ALWAYS AS (substr(timestamp, 1, 10)) VIRTUAL,
		archived INTEGER DEFAULT 0
	) STRICT;

	CREATE INDEX IF NOT EXISTS idx_records_project_date ON records(project, date) WHERE archived = 0;

	CREATE TABLE IF NOT EXISTS daily_budgets (
		date TEXT,
		project TEXT,
		total_millicents INTEGER DEFAULT 0,
		request_count INTEGER DEFAULT 0,
		model_stats TEXT DEFAULT '{}', 
		PRIMARY KEY(date, project)
	) STRICT;
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *sqliteStore) Write(ctx context.Context, r Record) error {
	if s.closed.Load() {
		return fmt.Errorf("store is closed")
	}
	select {
	case s.queue <- r:
		return nil
	default:
		return fmt.Errorf("write queue full")
	}
}

func (s *sqliteStore) worker() {
	defer s.wg.Done()
	batch := make([]Record, 0, 100)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.flushBatch(batch); err != nil {
			log.Printf("[store] flush failed: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case r, ok := <-s.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, r)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *sqliteStore) flushBatch(batch []Record) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO records (id, trace_id, project, model, input_tokens, output_tokens, cost_millicents, route_target, duration_ms, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range batch {
		tsStr := r.Timestamp.UTC().Format("2006-01-02 15:04:05")
		dateStr := tsStr[:10]
		if _, err := stmt.Exec(r.ID.String(), r.TraceID, r.Project, r.Model, r.InputTokens, r.OutputTokens, r.CostMillicents, r.RouteTarget, r.DurationMs, tsStr); err != nil {
			return err
		}

		upsertSQL := `
		INSERT INTO daily_budgets (date, project, total_millicents, request_count, model_stats)
		VALUES (?, ?, ?, 1, json_object(?, 1))
		ON CONFLICT(date, project) DO UPDATE SET
			total_millicents = total_millicents + EXCLUDED.total_millicents,
			request_count = request_count + 1,
			model_stats = json_set(
				COALESCE(model_stats, '{}'),
				'$."' || ? || '"',
				COALESCE(json_extract(model_stats, '$."' || ? || '"'), 0) + 1
			);
		`
		if _, err := tx.Exec(upsertSQL, dateStr, r.Project, r.CostMillicents, r.Model, r.Model, r.Model); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteStore) GetDailyStats(ctx context.Context, project string, date string) (DailyStats, error) {
	var stats DailyStats
	var modelStatsJSON sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT date, project, total_millicents, request_count, model_stats FROM daily_budgets WHERE project = ? AND date = ?`, project, date).Scan(&stats.Date, &stats.Project, &stats.TotalMillicents, &stats.RequestCount, &modelStatsJSON)
	if err == sql.ErrNoRows {
		return DailyStats{Date: date, Project: project, ModelBreakdown: make(map[string]int)}, nil
	}
	if err != nil {
		return stats, err
	}
	stats.ModelBreakdown = make(map[string]int)
	if modelStatsJSON.Valid && modelStatsJSON.String != "" && modelStatsJSON.String != "{}" {
		_ = json.Unmarshal([]byte(modelStatsJSON.String), &stats.ModelBreakdown)
	}
	return stats, nil
}

func (s *sqliteStore) GetRecentRequests(ctx context.Context, project string, limit int) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, trace_id, project, model, input_tokens, output_tokens, cost_millicents, route_target, duration_ms, timestamp FROM records WHERE project = ? AND archived = 0 ORDER BY timestamp DESC LIMIT ?`, project, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Record
	for rows.Next() {
		var r Record
		var idStr, tsStr string
		if err := rows.Scan(&idStr, &r.TraceID, &r.Project, &r.Model, &r.InputTokens, &r.OutputTokens, &r.CostMillicents, &r.RouteTarget, &r.DurationMs, &tsStr); err != nil {
			return nil, err
		}
		r.ID, _ = uuid.Parse(idStr)
		r.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		results = append(results, r)
	}
	return results, nil
}

func (s *sqliteStore) QueryProjects(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT project FROM daily_budgets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *sqliteStore) Archive(ctx context.Context, beforeDate string) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM records WHERE date < ?`, beforeDate)
	if err != nil {
		return 0, err
	}
	count, _ := res.RowsAffected()
	_, _ = s.db.ExecContext(ctx, "VACUUM")
	return int(count), nil
}

func (s *sqliteStore) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(s.queue)
	s.wg.Wait()
	return s.db.Close()
}
