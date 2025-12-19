package policy

import "github.com/bosbaber/hackweek/microvault/internal/core"

// Engine evaluates authorization decisions based on balances.
type Engine struct {
	MinIngressCost int64
}

// New returns an Engine with the specified minimum ingress cost.
func New(minIngressCost int64) Engine {
	return Engine{MinIngressCost: minIngressCost}
}

func (e Engine) IsFrozen(balance int64) bool {
	return core.DeriveState(balance) == core.AccountStateFrozen
}

func (e Engine) CanUpload(balance int64) bool {
	if e.IsFrozen(balance) {
		return false
	}
	return balance >= e.MinIngressCost
}

func (e Engine) CanDownload(balance int64) bool {
	return !e.IsFrozen(balance)
}
