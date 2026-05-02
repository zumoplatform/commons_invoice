package commons_invoice

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SearchFilters are the optional filters accepted by Repo.Search. Leave any
// field at its zero value (or nil pointer) to skip that predicate.
type SearchFilters struct {
	Customer  string     // substring match against customers.name
	Status    string     // exact match — must be one of the CHECK values
	Statuses  []string   // status IN (...) — wins over Status when both set
	DateFrom  *time.Time // issue_date >= DateFrom
	DateTo    *time.Time // issue_date <= DateTo
	AmountMin *float64   // amount >= AmountMin
	AmountMax *float64   // amount <= AmountMax
}

type Repo struct {
	DB *gorm.DB
}

func NewRepo(db *gorm.DB) Repo {
	return Repo{DB: db}
}

func (r Repo) ListByOrganizationID(organizationID int64) ([]Invoice, error) {
	var invoices []Invoice
	if err := r.DB.
		Where("organization_id = ?", organizationID).
		Order("issue_date DESC, id DESC").
		Find(&invoices).Error; err != nil {
		return nil, err
	}
	return invoices, nil
}

func (r Repo) GetByID(organizationID, invoiceID int64) (*Invoice, error) {
	var inv Invoice
	if err := r.DB.
		Preload("Customer").
		Preload("Organization").
		Preload("Items.Product").
		Where("organization_id = ? AND id = ?", organizationID, invoiceID).
		First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

// Recalculate refreshes the invoice's derived totals.
//
//   - When the invoice has line items, subtotal is overwritten with the sum
//     of item subtotals. Each item.subtotal already accounts for its own
//     line-level discount (commons_invoice_item handles that math), so the
//     sum is the true post-discount pre-tax total. Item-less invoices keep
//     whatever subtotal was last set (so the "$1500 invoice with no items"
//     flow still works).
//   - amount is always recomputed: subtotal + tax (clamped to 0).
//
// Tax is not modified — apply it via Update first, then call Recalculate.
// Idempotent.
func (r Repo) Recalculate(organizationID, invoiceID int64) (*Invoice, error) {
	inv, err := r.GetByID(organizationID, invoiceID)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}

	if len(inv.Items) > 0 {
		var subtotal float64
		for _, it := range inv.Items {
			subtotal += it.Subtotal
		}
		if subtotal != inv.Subtotal {
			inv.Subtotal = subtotal
			updates["subtotal"] = subtotal
		}
	}

	amount := inv.Subtotal + inv.Tax
	if amount < 0 {
		amount = 0
	}
	if amount != inv.Amount {
		inv.Amount = amount
		updates["amount"] = amount
	}

	if len(updates) > 0 {
		if err := r.DB.Model(inv).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return inv, nil
}

// Search returns invoices matching every non-zero filter in f, ordered by
// issue_date descending. When f.Customer is set, joins customers and does a
// case-insensitive substring match on name.
func (r Repo) Search(organizationID int64, f SearchFilters) ([]Invoice, error) {
	q := r.DB.Where("invoices.organization_id = ?", organizationID)
	if len(f.Statuses) > 0 {
		q = q.Where("invoices.status IN ?", f.Statuses)
	} else if f.Status != "" {
		q = q.Where("invoices.status = ?", f.Status)
	}
	if f.DateFrom != nil {
		q = q.Where("invoices.issue_date >= ?", *f.DateFrom)
	}
	if f.DateTo != nil {
		q = q.Where("invoices.issue_date <= ?", *f.DateTo)
	}
	if f.AmountMin != nil {
		q = q.Where("invoices.amount >= ?", *f.AmountMin)
	}
	if f.AmountMax != nil {
		q = q.Where("invoices.amount <= ?", *f.AmountMax)
	}
	if f.Customer != "" {
		q = q.Joins("JOIN customers ON customers.id = invoices.customer_id").
			Where("customers.name ILIKE ?", "%"+f.Customer+"%")
	}
	var invoices []Invoice
	if err := q.Order("invoices.issue_date DESC, invoices.id DESC").Find(&invoices).Error; err != nil {
		return nil, err
	}
	return invoices, nil
}

// CreateInput carries the fields accepted on invoice creation. The row's
// bigserial id is the canonical reference — there's no separate
// invoice_number anymore. Discounts now live exclusively on line items
// (commons_invoice_item.InvoiceItem.Discount), so there's no whole-invoice
// discount field here.
type CreateInput struct {
	OrganizationID int64
	CustomerID     int64
	Status         string // empty defaults to "draft"
	IssueDate      time.Time
	DueDate        *time.Time
	Subtotal       float64
	Tax            float64
	Amount         float64
	Notes          string
}

// Create inserts a new invoice. Status defaults to "draft" when blank;
// any non-empty value must be a valid Status.
func (r Repo) Create(in CreateInput) (*Invoice, error) {
	if in.OrganizationID == 0 {
		return nil, fmt.Errorf("organization_id is required")
	}
	if in.CustomerID == 0 {
		return nil, fmt.Errorf("customer_id is required")
	}

	status := in.Status
	if status == "" {
		status = string(StatusDraft)
	}
	if !Status(status).IsValid() {
		return nil, ErrInvalidStatus{Status: Status(status)}
	}

	if in.IssueDate.IsZero() {
		in.IssueDate = time.Now()
	}

	inv := Invoice{
		OrganizationID: in.OrganizationID,
		CustomerID:     in.CustomerID,
		Status:         status,
		IssueDate:      in.IssueDate,
		DueDate:        in.DueDate,
		Subtotal:       in.Subtotal,
		Tax:            in.Tax,
		Amount:         in.Amount,
		Notes:          in.Notes,
	}
	if err := r.DB.Create(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

// UpdateInput is the partial-update payload. nil pointer = leave field alone.
// Status is FSM-validated against the row's current value; everything else
// is applied verbatim. No Discount field — line-item discounts only.
type UpdateInput struct {
	CustomerID *int64
	Status     *string
	IssueDate  *time.Time
	DueDate    *time.Time
	Subtotal   *float64
	Tax        *float64
	Amount     *float64
	Notes      *string
}

// Update applies in to the invoice (organizationID, invoiceID). When
// in.Status is set, the FSM is checked against the current status before any
// field is changed. Returns the post-update row, with Items preloaded.
func (r Repo) Update(organizationID, invoiceID int64, in UpdateInput) (*Invoice, error) {
	var inv Invoice
	if err := r.DB.
		Where("organization_id = ? AND id = ?", organizationID, invoiceID).
		First(&inv).Error; err != nil {
		return nil, err
	}

	updates := map[string]any{}

	if in.Status != nil {
		next := Status(*in.Status)
		if !next.IsValid() {
			return nil, ErrInvalidStatus{Status: next}
		}
		current := Status(inv.Status)
		if current != next && !current.CanTransitionTo(next) {
			return nil, ErrInvalidTransition{From: current, To: next}
		}
		updates["status"] = string(next)
	}
	if in.CustomerID != nil {
		updates["customer_id"] = *in.CustomerID
	}
	if in.IssueDate != nil {
		updates["issue_date"] = *in.IssueDate
	}
	if in.DueDate != nil {
		updates["due_date"] = *in.DueDate
	}
	if in.Subtotal != nil {
		updates["subtotal"] = *in.Subtotal
	}
	if in.Tax != nil {
		updates["tax"] = *in.Tax
	}
	if in.Amount != nil {
		updates["amount"] = *in.Amount
	}
	if in.Notes != nil {
		updates["notes"] = *in.Notes
	}

	if len(updates) > 0 {
		if err := r.DB.Model(&inv).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return r.GetByID(organizationID, invoiceID)
}

// Delete removes an invoice scoped to organizationID. The DB-level FK on
// invoice_items.invoice_id is ON DELETE CASCADE (migration 00004), so a
// single DELETE wipes the invoice and every line item attached to it.
//
// Returns gorm.ErrRecordNotFound when no row matched (so the handler can
// surface a 404 rather than a silent 200 on missing/cross-org ids).
func (r Repo) Delete(organizationID, invoiceID int64) error {
	res := r.DB.
		Where("organization_id = ? AND id = ?", organizationID, invoiceID).
		Delete(&Invoice{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus transitions an invoice to next, validating both that next is a
// known status and that the move is allowed from the current status.
func (r Repo) UpdateStatus(organizationID, invoiceID int64, next Status) (*Invoice, error) {
	if !next.IsValid() {
		return nil, ErrInvalidStatus{Status: next}
	}
	var inv Invoice
	if err := r.DB.
		Where("organization_id = ? AND id = ?", organizationID, invoiceID).
		First(&inv).Error; err != nil {
		return nil, err
	}
	current := Status(inv.Status)
	if current == next {
		return &inv, nil
	}
	if !current.CanTransitionTo(next) {
		return nil, ErrInvalidTransition{From: current, To: next}
	}
	inv.Status = string(next)
	if err := r.DB.Save(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}
