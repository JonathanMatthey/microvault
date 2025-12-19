package ledger

// Ledger is the accounting interface for credit balances.
type Ledger interface {
	Balance(userID string) int64
	Credit(userID string, amount int64) error
	Debit(userID string, amount int64) error
	ListAll() (map[string]int64, error)
	RecordTransaction(userID string, txn Transaction) error
	GetTransactionHistory(userID string, limit int) ([]Transaction, error)
}
