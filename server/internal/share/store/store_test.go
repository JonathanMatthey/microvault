// ...existing code...
package store

import (
	"testing"
	"time"
)

func TestMemoryStore_Basic(t *testing.T) {
	s := NewMemory()
	link, err := s.Create("user1", "file1", 2*time.Second)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if link.OwnerID != "user1" || link.UploadID != "file1" {
		t.Errorf("Link fields wrong: %+v", link)
	}
	got, ok, err := s.Get(link.Token)
	if err != nil || !ok {
		t.Fatalf("Get failed: %v %v", ok, err)
	}
	if got.Token != link.Token {
		t.Errorf("Token mismatch")
	}
	s.IncrementDownloads(link.Token)
	got2, ok, _ := s.Get(link.Token)
	if got2.Downloads != 1 {
		t.Errorf("Downloads not incremented")
	}
	// Expiry
	time.Sleep(2100 * time.Millisecond)
	_, ok, _ = s.Get(link.Token)
	if ok {
		t.Errorf("Link should be expired")
	}
}
