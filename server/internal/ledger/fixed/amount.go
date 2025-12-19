package fixed

import (
	"errors"
	"math"
)

var (
	ErrOverflow  = errors.New("overflow")
	ErrUnderflow = errors.New("underflow")
)

// Add adds two scaled int64 amounts with overflow protection.
func Add(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, ErrOverflow
	}
	return a + b, nil
}

// Sub subtracts b from a with overflow/underflow protection.
func Sub(a, b int64) (int64, error) {
	return Add(a, -b)
}
