package account

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/ledger"
)

// HandleGetTransactions returns up to N recent transactions (newest first).
func HandleGetTransactions(l ledger.Ledger) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing user ID"})
		}

		limit := 50
		if raw := c.QueryParam("limit"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}

		txns, err := l.GetTransactionHistory(userID, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch transactions"})
		}

		return c.JSON(http.StatusOK, echo.Map{
			"transactions": txns,
			"count":        len(txns),
		})
	}
}
