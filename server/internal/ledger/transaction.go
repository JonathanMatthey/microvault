package ledger

import "time"

// Transaction captures a single credit or debit event for a user.
type Transaction struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Type         string                 `json:"type"`          // "credit" or "debit"
	Reason       string                 `json:"reason"`        // e.g., "storage", "ingress", "egress", "payment", "manual"
	Amount       int64                  `json:"amount"`        // positive = credit, negative = debit
	BalanceAfter int64                  `json:"balance_after"` // balance immediately after this transaction
	Description  string                 `json:"description"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
