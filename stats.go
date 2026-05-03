package commons_invoice

// stats.go aggregates per-organization revenue numbers off the invoices
// table. All calculations live here so the handler stays a thin wrapper
// and the chat tool / cron / admin endpoints can call into the same
// shape later without re-implementing the SQL.

import "fmt"

// OrgStats is the snapshot returned by Repo.OrgStats. All currency
// fields are USD; counts are unsigned in spirit (Postgres returns int64).
//
// Field semantics:
//   - TotalPaid:        lifetime sum of amount across status='paid' rows.
//   - TotalOutstanding: sent + overdue (everything you've billed that
//                       hasn't been collected, not counting drafts/voids).
//   - PaidCount / OutstandingCount: row counts mirroring the above.
//   - PaidLast30Days:   sum of amount for invoices that flipped to
//                       'paid' in the trailing 30 days. Uses
//                       inventory_deducted_at as the canonical "marked
//                       paid" timestamp; falls back to updated_at for
//                       rows that pre-date that column (NULL there).
//   - TopCustomers:     5 highest-paying customers, name + email +
//                       sum(amount) + count(*).
type OrgStats struct {
	TotalPaid         float64           `json:"total_paid"`
	TotalOutstanding  float64           `json:"total_outstanding"`
	PaidCount         int64             `json:"paid_count"`
	OutstandingCount  int64             `json:"outstanding_count"`
	PaidLast30Days    float64           `json:"paid_last_30_days"`
	PaidLast30Count   int64             `json:"paid_last_30_count"`
	TopCustomers      []TopCustomerStat `json:"top_customers"`
}

// TopCustomerStat is one row of the leaderboard.
type TopCustomerStat struct {
	CustomerID int64   `json:"customer_id"`
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	PaidTotal  float64 `json:"paid_total"`
	PaidCount  int64   `json:"paid_count"`
}

// OrgStats computes the snapshot in two SQL round-trips: one for the
// scalar aggregates (FILTER clauses keep it to a single pass) and one
// for the customer leaderboard (joined to customers for name + email).
//
// Why two queries: combining them would force a window function or a
// subquery just to bring scalar totals alongside per-customer rows;
// two simple queries are clearer and the Postgres planner doesn't
// reward the contortions.
func (r Repo) OrgStats(organizationID int64) (*OrgStats, error) {
	if organizationID == 0 {
		return nil, fmt.Errorf("organization_id is required")
	}

	stats := OrgStats{TopCustomers: []TopCustomerStat{}}

	// Scalar aggregates.
	row := r.DB.Raw(`
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE status = 'paid'), 0)                                                                                AS total_paid,
			COALESCE(SUM(amount) FILTER (WHERE status IN ('sent', 'overdue')), 0)                                                                  AS total_outstanding,
			COUNT(*) FILTER (WHERE status = 'paid')                                                                                                AS paid_count,
			COUNT(*) FILTER (WHERE status IN ('sent', 'overdue'))                                                                                  AS outstanding_count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'paid' AND COALESCE(inventory_deducted_at, updated_at) >= now() - interval '30 days'), 0) AS paid_last_30_days,
			COUNT(*)            FILTER (WHERE status = 'paid' AND COALESCE(inventory_deducted_at, updated_at) >= now() - interval '30 days')      AS paid_last_30_count
		FROM invoices
		WHERE organization_id = ?
	`, organizationID).Row()
	if err := row.Scan(
		&stats.TotalPaid,
		&stats.TotalOutstanding,
		&stats.PaidCount,
		&stats.OutstandingCount,
		&stats.PaidLast30Days,
		&stats.PaidLast30Count,
	); err != nil {
		return nil, fmt.Errorf("scan stats: %w", err)
	}

	// Top customers by paid revenue. Limit 5 — anything beyond that
	// belongs in a dedicated reports page, not the dashboard widget.
	if err := r.DB.Raw(`
		SELECT
			c.id        AS customer_id,
			c.name      AS name,
			c.email     AS email,
			SUM(i.amount) AS paid_total,
			COUNT(*)      AS paid_count
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		WHERE i.organization_id = ? AND i.status = 'paid'
		GROUP BY c.id, c.name, c.email
		ORDER BY paid_total DESC, c.name ASC
		LIMIT 5
	`, organizationID).Scan(&stats.TopCustomers).Error; err != nil {
		return nil, fmt.Errorf("scan top customers: %w", err)
	}
	return &stats, nil
}
