package memory

import (
	"sync"
	"testing"
)

func TestLedgerCreditDebit(t *testing.T) {
	l := New()
	if err := l.Credit("user", 1_000); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if got := l.Balance("user"); got != 1_000 {
		t.Fatalf("balance want 1000 got %d", got)
	}
	if err := l.Debit("user", 500); err != nil {
		t.Fatalf("debit: %v", err)
	}
	if got := l.Balance("user"); got != 500 {
		t.Fatalf("balance want 500 got %d", got)
	}
}

func TestLedgerAllowsNegative(t *testing.T) {
	l := New()
	if err := l.Debit("user", 200); err != nil {
		t.Fatalf("debit: %v", err)
	}
	if got := l.Balance("user"); got != -200 {
		t.Fatalf("balance want -200 got %d", got)
	}
}

func TestLedgerRejectsNegativeAmount(t *testing.T) {
	l := New()
	if err := l.Credit("user", -1); err == nil {
		t.Fatalf("expected error for negative credit")
	}
	if err := l.Debit("user", -1); err == nil {
		t.Fatalf("expected error for negative debit")
	}
}

func TestLedgerConcurrentDebits(t *testing.T) {
	l := New()
	if err := l.Credit("user", 1_000_000); err != nil {
		t.Fatalf("credit: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			_ = l.Debit("user", 10_000)
		}()
	}
	wg.Wait()

	// Expect 100 * 10k debits = 1,000,000
	if got := l.Balance("user"); got != 0 {
		t.Fatalf("expected 0 balance, got %d", got)
	}
}
