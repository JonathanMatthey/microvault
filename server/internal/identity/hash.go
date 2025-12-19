package identity

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashUserID produces a deterministic, collision-resistant hash of a user ID.
func HashUserID(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(sum[:])
}
