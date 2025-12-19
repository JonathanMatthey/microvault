package identity

import (
	"context"
	"testing"
	"time"
)

func TestDevLoginAuthenticateSetsHashAndActivity(t *testing.T) {
	fixedTime := time.Unix(1700000000, 0)
	prov := &DevLoginProvider{Now: func() time.Time { return fixedTime }}
	ctx := WithUserID(context.Background(), "user@example.com")

	user, err := prov.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID != "user@example.com" {
		t.Fatalf("unexpected id: %s", user.ID)
	}
	if user.Hash != HashUserID(user.ID) {
		t.Fatalf("hash mismatch")
	}
	if user.LastActive != fixedTime.Unix() {
		t.Fatalf("last active mismatch: got %d", user.LastActive)
	}
}
