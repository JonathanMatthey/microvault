package identity

import "testing"

func TestHashUserIDDeterministic(t *testing.T) {
    h1 := HashUserID("alice@example.com")
    h2 := HashUserID("alice@example.com")
    if h1 != h2 {
        t.Fatalf("hash should be deterministic")
    }

    h3 := HashUserID("bob@example.com")
    if h1 == h3 {
        t.Fatalf("hash should differ for different users")
    }
}
