package commons_invoice

import "fmt"

type Status string

const (
	StatusDraft   Status = "draft"
	StatusSent    Status = "sent"
	StatusPaid    Status = "paid"
	StatusOverdue Status = "overdue"
	StatusVoid    Status = "void"
)

// statusTransitions maps each status to the set of statuses it may move to.
// Terminal statuses (paid, void) map to empty sets.
//
// draft → paid is allowed for in-person / cash sales: the invoice was
// drafted and paid on the spot, never emailed. Skipping "sent" in
// that case is intentional. The inventory auto-deduct still fires
// correctly — it gates on the destination being "paid", not on
// which prior status was in play.
var statusTransitions = map[Status]map[Status]bool{
	StatusDraft:   {StatusSent: true, StatusPaid: true, StatusVoid: true},
	StatusSent:    {StatusPaid: true, StatusOverdue: true, StatusVoid: true},
	StatusOverdue: {StatusPaid: true, StatusVoid: true},
	StatusPaid:    {},
	StatusVoid:    {},
}

func (s Status) IsValid() bool {
	_, ok := statusTransitions[s]
	return ok
}

func (s Status) CanTransitionTo(next Status) bool {
	allowed, ok := statusTransitions[s]
	if !ok {
		return false
	}
	return allowed[next]
}

type ErrInvalidStatus struct {
	Status Status
}

func (e ErrInvalidStatus) Error() string {
	return fmt.Sprintf("invalid status: %q", string(e.Status))
}

type ErrInvalidTransition struct {
	From, To Status
}

func (e ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid status transition: %s -> %s", e.From, e.To)
}
