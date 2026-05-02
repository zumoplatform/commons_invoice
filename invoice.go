package commons_invoice

import (
	"time"

	"github.com/zumoplatform/commons_customer"
	"github.com/zumoplatform/commons_invoice_item"
	"github.com/zumoplatform/commons_organization"
)

type Invoice struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	OrganizationID int64      `json:"organization_id" gorm:"not null;index"`
	CustomerID     int64      `json:"invoice_author" gorm:"column:customer_id;not null;index"`
	Status         string     `json:"status" gorm:"type:text;not null;default:'draft'"`
	IssueDate      time.Time  `json:"issue_date" gorm:"type:date;not null;default:CURRENT_DATE"`
	DueDate        *time.Time `json:"due_date" gorm:"type:date"`
	Subtotal       float64    `json:"subtotal" gorm:"type:numeric(12,2);not null;default:0"`
	Tax            float64    `json:"tax" gorm:"type:numeric(12,2);not null;default:0"`
	Amount         float64    `json:"amount" gorm:"type:numeric(12,2);not null;default:0"`
	Notes          string     `json:"notes" gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null;default:now()"`

	Organization *commons_organization.Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID;references:ID;constraint:OnDelete:CASCADE"`
	Customer     *commons_customer.Customer         `json:"customer,omitempty" gorm:"foreignKey:CustomerID;references:ID;constraint:OnDelete:RESTRICT"`
	Items        []commons_invoice_item.InvoiceItem `json:"items,omitempty" gorm:"foreignKey:InvoiceID;references:ID;constraint:OnDelete:CASCADE"`
}

func (Invoice) TableName() string {
	return "invoices"
}
