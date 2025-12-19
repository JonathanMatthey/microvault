package policy

import "testing"

func TestPolicyDecisions(t *testing.T) {
	minIngressCost := int64(100) // Test with 100 credit units minimum
	eng := New(minIngressCost)

	tests := []struct {
		name        string
		balance     int64
		canUpload   bool
		canDownload bool
		isFrozen    bool
	}{
		{"active_enough", eng.MinIngressCost + 1, true, true, false},
		{"active_exact", eng.MinIngressCost, true, true, false},
		{"active_but_low", eng.MinIngressCost - 1, false, true, false},
		{"frozen", -1, false, false, true},
	}

	for _, tc := range tests {
		c := tc
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := eng.CanUpload(c.balance); got != c.canUpload {
				t.Fatalf("CanUpload mismatch: got %v want %v", got, c.canUpload)
			}
			if got := eng.CanDownload(c.balance); got != c.canDownload {
				t.Fatalf("CanDownload mismatch: got %v want %v", got, c.canDownload)
			}
			if got := eng.IsFrozen(c.balance); got != c.isFrozen {
				t.Fatalf("IsFrozen mismatch: got %v want %v", got, c.isFrozen)
			}
		})
	}
}
