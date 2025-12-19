package fixed

import (
    "math"
    "testing"
)

func TestAddAndSub(t *testing.T) {
    t.Run("add simple", func(t *testing.T) {
        got, err := Add(10, 5)
        if err != nil {
            t.Fatalf("unexpected err: %v", err)
        }
        if got != 15 {
            t.Fatalf("want 15, got %d", got)
        }
    })

    t.Run("sub simple", func(t *testing.T) {
        got, err := Sub(10, 3)
        if err != nil {
            t.Fatalf("unexpected err: %v", err)
        }
        if got != 7 {
            t.Fatalf("want 7, got %d", got)
        }
    })

    t.Run("overflow", func(t *testing.T) {
        _, err := Add(math.MaxInt64, 1)
        if err != ErrOverflow {
            t.Fatalf("expected overflow, got %v", err)
        }
    })

    t.Run("underflow", func(t *testing.T) {
        _, err := Sub(math.MinInt64, 1)
        if err != ErrOverflow {
            t.Fatalf("expected overflow/underflow, got %v", err)
        }
    })
}
