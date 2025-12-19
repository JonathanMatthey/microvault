package core

import "testing"

func TestDeriveState(t *testing.T) {
	tests := []struct {
		balance int64
		expect  AccountState
	}{
		{0, AccountStateActive},
		{1, AccountStateActive},
		{-1, AccountStateFrozen},
	}

	for _, tc := range tests {
		if got := DeriveState(tc.balance); got != tc.expect {
			t.Fatalf("balance %d -> %s, want %s", tc.balance, got, tc.expect)
		}
	}
}

func TestCalculateIngressCost(t *testing.T) {
	// Test with rate of 15000 credit units per GiB (1.5 credits with scale=4, unit=10000)
	ingressPerGiB := int64(15000)

	tests := []struct {
		name  string
		bytes int64
		want  int64
	}{
		{"zero bytes", 0, 0},
		{"one byte", 1, 0},        // (1 * 15000) / 1073741824 = 0
		{"1 MB", 1024 * 1024, 14}, // (1048576 * 15000) / 1073741824 = 14.6... = 14
		{"1 GiB exact", GiBBytes, 15000},
		{"2 GiB", 2 * GiBBytes, 30000},
		{"100 MB", 100 * 1024 * 1024, 1464}, // (104857600 * 15000) / 1073741824 = 1464.8... = 1464
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateIngressCost(tc.bytes, ingressPerGiB)
			if got != tc.want {
				t.Errorf("CalculateIngressCost(%d, %d) = %d, want %d", tc.bytes, ingressPerGiB, got, tc.want)
			}
		})
	}
}

func TestCalculateEgressCost(t *testing.T) {
	// Test with rate of 30000 credit units per GiB (3.0 credits with scale=4, unit=10000)
	egressPerGiB := int64(30000)

	tests := []struct {
		name  string
		bytes int64
		want  int64
	}{
		{"zero bytes", 0, 0},
		{"one byte", 1, 0},        // (1 * 30000) / 1073741824 = 0
		{"1 MB", 1024 * 1024, 29}, // (1048576 * 30000) / 1073741824 = 29.2... = 29
		{"1 GiB exact", GiBBytes, 30000},
		{"2 GiB", 2 * GiBBytes, 60000},
		{"500 MB", 500 * 1024 * 1024, 14648}, // (524288000 * 30000) / 1073741824 = 14648.4... = 14648
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateEgressCost(tc.bytes, egressPerGiB)
			if got != tc.want {
				t.Errorf("CalculateEgressCost(%d, %d) = %d, want %d", tc.bytes, egressPerGiB, got, tc.want)
			}
		})
	}
}

func TestCalculateStorageMonthlyCost(t *testing.T) {
	// Test with rate of 10000 credit units per GiB-month (1.0 credit with scale=4)
	storagePerGiBMonth := int64(10000)

	tests := []struct {
		name  string
		bytes int64
		want  int64
	}{
		{"zero bytes", 0, 0},
		{"one byte", 1, 0},
		{"1 MB", 1024 * 1024, 9}, // (1_048_576 * 10000) / 1_073_741_824 = 9.7 -> 9
		{"1 GiB exact", GiBBytes, 10000},
		{"2 GiB", 2 * GiBBytes, 20000},
		{"500 MB", 500 * 1024 * 1024, 4882}, // (524_288_000 * 10000) / 1_073_741_824 = 4882.8 -> 4882
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateStorageMonthlyCost(tc.bytes, storagePerGiBMonth)
			if got != tc.want {
				t.Errorf("CalculateStorageMonthlyCost(%d, %d) = %d, want %d", tc.bytes, storagePerGiBMonth, got, tc.want)
			}
		})
	}
}
