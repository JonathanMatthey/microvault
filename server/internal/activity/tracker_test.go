package activity

import "testing"

func TestTrackerRecordsActivity(t *testing.T) {
    tk := New()
    tk.Record("user", 123)

    if got := tk.LastActive("user"); got != 123 {
        t.Fatalf("expected 123 got %d", got)
    }

    if got := tk.LastActive("missing"); got != 0 {
        t.Fatalf("expected zero for missing, got %d", got)
    }
}
