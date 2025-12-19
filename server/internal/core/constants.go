package core

import "time"

const (
	GiBBytes int64 = 1024 * 1024 * 1024 // 1 GiB in bytes
)

type AccountState string

const (
	AccountStateActive AccountState = "active"
	AccountStateFrozen AccountState = "frozen"
)

// DeriveState calculates the account state based on balance.
func DeriveState(balance int64) AccountState {
	if balance < 0 {
		return AccountStateFrozen
	}
	return AccountStateActive
}

// CalculateIngressCost calculates the credit cost for uploading bytes.
// Formula: (bytes * creditsPerGiB) / GiBBytes
func CalculateIngressCost(bytes int64, creditsPerGiB int64) int64 {
	if bytes <= 0 {
		return 0
	}
	cost := (bytes * creditsPerGiB) / GiBBytes
	if cost == 0 && creditsPerGiB > 0 {
		// Enforce a minimum non-zero charge to prevent abuse for tiny files
		return 1
	}
	return cost
}

// CalculateEgressCost calculates the credit cost for downloading bytes.
// Formula: (bytes * creditsPerGiB) / GiBBytes
func CalculateEgressCost(bytes int64, creditsPerGiB int64) int64 {
	if bytes <= 0 {
		return 0
	}
	cost := (bytes * creditsPerGiB) / GiBBytes
	if cost == 0 && creditsPerGiB > 0 {
		// Enforce a minimum non-zero charge to prevent abuse for tiny files
		return 1
	}
	return cost
}

// CalculateStorageMonthlyCost calculates the monthly storage cost for bytes at a rate per GiB-month.
func CalculateStorageMonthlyCost(bytes int64, creditsPerGiBMonth int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return (bytes * creditsPerGiBMonth) / GiBBytes
}

// CalculateStorageIntervalCost calculates the storage cost for an interval using a per-GiB interval rate.
func CalculateStorageIntervalCost(bytes int64, creditsPerGiBInterval int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return (bytes * creditsPerGiBInterval) / GiBBytes
}

// NowRFC3339 returns the current time in RFC3339 format
func NowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
