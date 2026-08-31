// cursor-billing-backfill 按 Cursor list 价 + CodexBar 分桶语义，重算历史 cursor-* usage_logs 费用，
// 并将多扣余额退还给用户（仅 billing_type=钱包余额）。
//
// 用法：
//
//	go run ./cmd/cursor-billing-backfill [--execute]
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const billingTypeBalance = int8(0)

type usageRow struct {
	id                        int64
	userID                    int64
	billingType               int8
	model                     string
	requestedModel            sql.NullString
	inputTokens               int
	outputTokens              int
	cacheCreationTokens       int
	cacheReadTokens           int
	cacheCreation5mTokens     int
	cacheCreation1hTokens     int
	imageInputTokens          int
	imageOutputTokens         int
	rateMultiplier            float64
	serviceTier               sql.NullString
	longContextBillingApplied bool
	inputCost                 float64
	outputCost                float64
	cacheCreationCost         float64
	cacheReadCost             float64
	imageInputCost            float64
	imageOutputCost           float64
	totalCost                 float64
	actualCost                float64
}

type userRefund struct {
	userID int64
	refund float64
	rows   int
}

func main() {
	execute := flag.Bool("execute", false, "apply updates (default dry-run)")
	flag.Parse()

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	client, db, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer func() { _ = client.Close() }()

	billing := service.NewBillingService(cfg, nil)

	ctx := context.Background()
	rows, err := loadCursorUsageRows(ctx, db)
	if err != nil {
		log.Fatalf("load rows: %v", err)
	}
	if len(rows) == 0 {
		fmt.Println("no cursor usage rows found")
		return
	}

	var (
		oldTotal, newTotal float64
		changed            int
		skipped            int
		refunds            = make(map[int64]*userRefund)
	)

	for i := range rows {
		row := &rows[i]
		model := billingModel(row)
		tokens := service.UsageTokens{
			InputTokens:           row.inputTokens,
			OutputTokens:          row.outputTokens,
			CacheCreationTokens:   row.cacheCreationTokens,
			CacheReadTokens:       row.cacheReadTokens,
			CacheCreation5mTokens: row.cacheCreation5mTokens,
			CacheCreation1hTokens: row.cacheCreation1hTokens,
			ImageInputTokens:      row.imageInputTokens,
			ImageOutputTokens:     row.imageOutputTokens,
		}
		serviceTier := ""
		if row.serviceTier.Valid {
			serviceTier = row.serviceTier.String
		}

		cost, err := billing.CalculateCostWithServiceTier(model, tokens, row.rateMultiplier, serviceTier)
		if err != nil {
			log.Printf("skip id=%d model=%q: %v", row.id, model, err)
			skipped++
			continue
		}

		oldTotal += row.actualCost
		newTotal += cost.ActualCost

		delta := row.actualCost - cost.ActualCost
		if nearlyEqual(row.actualCost, cost.ActualCost) &&
			nearlyEqual(row.inputCost, cost.InputCost) &&
			nearlyEqual(row.outputCost, cost.OutputCost) &&
			nearlyEqual(row.cacheReadCost, cost.CacheReadCost) &&
			nearlyEqual(row.cacheCreationCost, cost.CacheCreationCost) {
			continue
		}
		changed++

		if row.billingType == billingTypeBalance && delta > 0 {
			ur := refunds[row.userID]
			if ur == nil {
				ur = &userRefund{userID: row.userID}
				refunds[row.userID] = ur
			}
			ur.refund += delta
			ur.rows++
		}

		if *execute {
			if err := updateUsageLog(ctx, db, row.id, cost); err != nil {
				log.Fatalf("update usage_log id=%d: %v", row.id, err)
			}
		}
	}

	mode := "dry-run"
	if *execute {
		mode = "execute"
	}
	fmt.Printf("mode=%s rows=%d changed=%d skipped=%d old_actual_total=%.6f new_actual_total=%.6f refund_total=%.6f\n",
		mode, len(rows), changed, skipped, oldTotal, newTotal, oldTotal-newTotal)

	for _, ur := range refunds {
		fmt.Printf("user_id=%d refund=%.6f rows=%d\n", ur.userID, ur.refund, ur.rows)
	}

	if !*execute {
		fmt.Println("dry-run only; pass --execute to apply")
		return
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("begin refund tx: %v", err)
	}
	for _, ur := range refunds {
		if ur.refund <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE users SET balance = balance + $1, updated_at = NOW()
			WHERE id = $2`, ur.refund, ur.userID); err != nil {
			_ = tx.Rollback()
			log.Fatalf("refund user_id=%d: %v", ur.userID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit refunds: %v", err)
	}
	fmt.Println("backfill complete")
}

func loadCursorUsageRows(ctx context.Context, db *sql.DB) ([]usageRow, error) {
	query := `
		SELECT id, user_id, billing_type, model, requested_model,
		       input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		       cache_creation_5m_tokens, cache_creation_1h_tokens,
		       image_input_tokens, image_output_tokens,
		       rate_multiplier, service_tier, long_context_billing_applied,
		       input_cost, output_cost, cache_creation_cost, cache_read_cost,
		       image_input_cost, image_output_cost, total_cost, actual_cost
		FROM usage_logs
		WHERE model ILIKE 'cursor-%' OR requested_model ILIKE 'cursor-%'
		ORDER BY id ASC`
	rs, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var out []usageRow
	for rs.Next() {
		var row usageRow
		if err := rs.Scan(
			&row.id, &row.userID, &row.billingType, &row.model, &row.requestedModel,
			&row.inputTokens, &row.outputTokens, &row.cacheCreationTokens, &row.cacheReadTokens,
			&row.cacheCreation5mTokens, &row.cacheCreation1hTokens,
			&row.imageInputTokens, &row.imageOutputTokens,
			&row.rateMultiplier, &row.serviceTier, &row.longContextBillingApplied,
			&row.inputCost, &row.outputCost, &row.cacheCreationCost, &row.cacheReadCost,
			&row.imageInputCost, &row.imageOutputCost, &row.totalCost, &row.actualCost,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func billingModel(row *usageRow) string {
	if row.requestedModel.Valid {
		if m := strings.TrimSpace(row.requestedModel.String); m != "" {
			return m
		}
	}
	return row.model
}

func updateUsageLog(ctx context.Context, db *sql.DB, id int64, cost *service.CostBreakdown) error {
	_, err := db.ExecContext(ctx, `
		UPDATE usage_logs SET
			input_cost = $1,
			output_cost = $2,
			cache_creation_cost = $3,
			cache_read_cost = $4,
			image_input_cost = $5,
			image_output_cost = $6,
			total_cost = $7,
			actual_cost = $8,
			long_context_billing_applied = $9
		WHERE id = $10`,
		cost.InputCost, cost.OutputCost, cost.CacheCreationCost, cost.CacheReadCost,
		cost.ImageInputCost, cost.ImageOutputCost, cost.TotalCost, cost.ActualCost,
		cost.LongContextBillingApplied, id,
	)
	return err
}

func nearlyEqual(a, b float64) bool {
	const eps = 1e-9
	if a > b {
		return a-b < eps
	}
	return b-a < eps
}
