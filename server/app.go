package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/activity"
	"github.com/bosbaber/hackweek/microvault/internal/core"
	"github.com/bosbaber/hackweek/microvault/internal/identity"
	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/policy"
)

// app holds all core services
type app struct {
	ledger   ledger.Ledger
	policy   policy.Engine
	activity *activity.Tracker
}

// Request/response types
type amountRequest struct {
	Amount int64 `json:"amount"`
}

// withUser extracts the caller identity (dev or auth-disabled: X-User-ID; prod: Google ID token) and records activity.
func (a *app) withUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var userID string
		cfg := GetConfig()

		if DevMode || (cfg != nil && !cfg.Auth.Enabled) {
			userID = c.Request().Header.Get("X-User-ID")
			if userID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing X-User-ID header"})
			}
		} else {
			auth := c.Request().Header.Get("Authorization")
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			}
			user, err := VerifyGoogleToken(parts[1])
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}
			userID = user.Email
		}

		hash := identity.HashUserID(userID)
		now := time.Now().Unix()
		a.activity.Record(userID, now)

		c.Set("userID", userID)
		c.Set("userHash", hash)
		c.Set("lastActive", now)
		return next(c)
	}
}

func (a *app) handleLogin(c echo.Context) error {
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing X-User-ID header"})
	}

	user := identity.User{
		ID:         userID,
		Hash:       identity.HashUserID(userID),
		LastActive: time.Now().Unix(),
	}
	a.activity.Record(user.ID, user.LastActive)

	return c.JSON(http.StatusOK, user)
}

// handleUser returns basic profile and credits for the current user.
func (a *app) handleUser(c echo.Context) error {
	userID := c.Get("userID").(string)
	credits := a.ledger.Balance(userID)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"email":   userID,
		"id":      userID,
		"credits": credits,
	})
}

func (a *app) handleBalance(c echo.Context) error {
	userID := c.Get("userID").(string)
	balance := a.ledger.Balance(userID)
	state := core.DeriveState(balance)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":     userID,
		"hash":        c.Get("userHash").(string),
		"balance":     balance,
		"state":       state,
		"last_active": a.activity.LastActive(userID),
	})
}

func (a *app) handleCredit(c echo.Context) error {
	userID := c.Get("userID").(string)
	var req amountRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
	}
	if err := a.ledger.Credit(userID, req.Amount); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	balance := a.ledger.Balance(userID)
	_ = a.ledger.RecordTransaction(userID, ledger.Transaction{
		ID:           uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Type:         "credit",
		Reason:       "manual",
		Amount:       req.Amount,
		BalanceAfter: balance,
		Description:  "Manual credit",
	})
	return c.JSON(http.StatusOK, map[string]interface{}{"balance": balance})
}

func (a *app) handleDebit(c echo.Context) error {
	userID := c.Get("userID").(string)
	var req amountRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
	}
	if err := a.ledger.Debit(userID, req.Amount); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	balance := a.ledger.Balance(userID)
	_ = a.ledger.RecordTransaction(userID, ledger.Transaction{
		ID:           uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Type:         "debit",
		Reason:       "manual",
		Amount:       -req.Amount,
		BalanceAfter: balance,
		Description:  "Manual debit",
	})
	return c.JSON(http.StatusOK, map[string]interface{}{"balance": balance})
}

func (a *app) handlePolicy(c echo.Context) error {
	userID := c.Get("userID").(string)
	balance := a.ledger.Balance(userID)
	isFrozen := a.policy.IsFrozen(balance)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":              userID,
		"balance":              balance,
		"is_frozen":            isFrozen,
		"can_upload":           a.policy.CanUpload(balance),
		"can_download":         a.policy.CanDownload(balance),
		"min_ingress_required": a.policy.MinIngressCost,
	})
}

func (a *app) handleDevBoost(c echo.Context) error {
	userID := c.Get("userID").(string)
	// Add 5 credits (50000 internal units)
	boostAmount := int64(50000)
	if err := a.ledger.Credit(userID, boostAmount); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	newBalance := a.ledger.Balance(userID)
	_ = a.ledger.RecordTransaction(userID, ledger.Transaction{
		ID:           uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Type:         "credit",
		Reason:       "manual",
		Amount:       boostAmount,
		BalanceAfter: newBalance,
		Description:  "Dev boost",
	})
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Account boosted with 5 credits",
		"balance": newBalance,
		"user_id": userID,
	})
}
