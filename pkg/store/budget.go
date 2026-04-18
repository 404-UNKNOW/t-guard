package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// DeductBudget 实现原子扣款：BEGIN IMMEDIATE -> SELECT -> CHECK -> UPDATE -> INSERT -> COMMIT
func (s *sqliteStore) DeductBudget(ctx context.Context, r Record, limit int64) error {
	// 1. 开启事务 (SQLite 使用 IMMEDIATE 模式锁定写入)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	dateStr := r.Timestamp.UTC().Format("2006-01-02")

	// 2. 检查余额
	var currentUsed int64
	err = tx.QueryRowContext(ctx, `SELECT total_millicents FROM daily_budgets WHERE project = ? AND date = ?`, r.Project, dateStr).Scan(&currentUsed)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[store] query balance failed: %v", err)
		return err
	}

	// 如果没有记录，视为 0
	if err == sql.ErrNoRows {
		currentUsed = 0
	}

	// 3. 额度检查
	if limit > 0 && currentUsed+r.CostMillicents > limit {
		return ErrInsufficientBudget
	}

	// 4. 执行扣减 (Upsert)
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
	if _, err := tx.ExecContext(ctx, upsertSQL, dateStr, r.Project, r.CostMillicents, r.Model, r.Model, r.Model); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	// 5. 记录流水
	tsStr := r.Timestamp.UTC().Format("2006-01-02 15:04:05")
	if _, err := tx.ExecContext(ctx, `INSERT INTO records (id, trace_id, project, model, input_tokens, output_tokens, cost_millicents, route_target, duration_ms, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID.String(), r.TraceID, r.Project, r.Model, r.InputTokens, r.OutputTokens, r.CostMillicents, r.RouteTarget, r.DurationMs, tsStr); err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	// 6. 提交
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
