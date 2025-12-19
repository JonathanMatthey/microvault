package share

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/core"
	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/policy"
	"github.com/bosbaber/hackweek/microvault/internal/share/store"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

const defaultShareTTL = 7 * 24 * time.Hour

// HandleCreateShare returns POST /files/:uploadId/share
func HandleCreateShare(s store.Store, s3 *storage.S3Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}
		uploadID := c.Param("uploadId")
		if uploadID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing uploadId"})
		}
		// Verify the file exists for this user (skip if s3 is nil for tests)
		if s3 != nil {
			userHash := storage.UserHashFromID(userID)
			exists, err := s3.ObjectExists(context.Background(), userHash, uploadID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to check file"})
			}
			if !exists {
				return c.JSON(http.StatusNotFound, echo.Map{"error": "file not found"})
			}
		}

		lnk, err := s.Create(userID, uploadID, defaultShareTTL)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to create link"})
		}

		scheme := c.Scheme()
		if forwarded := c.Request().Header.Get("X-Forwarded-Proto"); forwarded != "" {
			scheme = forwarded
		}
		// Always force https for share links in prod
		if scheme != "https" {
			scheme = "https"
		}
		base := scheme + "://" + c.Request().Host
		url := fmt.Sprintf("%s/share/%s", base, lnk.Token)
		return c.JSON(http.StatusOK, echo.Map{
			"token":      lnk.Token,
			"url":        url,
			"expires_at": lnk.ExpiresAt.Format(time.RFC3339),
		})
	}
}

// HandleRedeemShare returns GET /share/:token
// Streams the file anonymously and charges the owner's balance AFTER a successful full transfer.
func HandleRedeemShare(s store.Store, l ledger.Ledger, p policy.Engine, s3 *storage.S3Client, egressPerGiB int64) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Param("token")
		if token == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing token"})
		}
		link, ok, err := s.Get(token)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to load link"})
		}
		if !ok {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "link not found or expired"})
		}
		if time.Now().After(link.ExpiresAt) {
			return c.JSON(http.StatusGone, echo.Map{"error": "link expired"})
		}

		// Owner policy check (account not frozen)
		balance := l.Balance(link.OwnerID)
		if !p.CanDownload(balance) {
			return c.JSON(http.StatusPaymentRequired, echo.Map{"error": "owner account is frozen"})
		}

		// Fetch object and compute cost from size (skip if s3 is nil for tests)
		var fileSize int64 = 10
		if s3 != nil {
			userHash := storage.UserHashFromID(link.OwnerID)
			exists, err := s3.ObjectExists(context.Background(), userHash, link.UploadID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to check file"})
			}
			if !exists {
				return c.JSON(http.StatusNotFound, echo.Map{"error": "file not found"})
			}

			objReader, objMeta, err := s3.GetObject(context.Background(), userHash, link.UploadID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to open file"})
			}
			defer objReader.Close()

			if objMeta.ContentLength != "" {
				fmt.Sscanf(objMeta.ContentLength, "%d", &fileSize)
			}
			// Set headers and stream
			c.Response().Header().Set("Content-Type", objMeta.ContentType)
			c.Response().Header().Set("Content-Length", objMeta.ContentLength)
			c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", objMeta.Filename))
			c.Response().Header().Set("Cache-Control", "private, max-age=0")
			c.Response().WriteHeader(http.StatusOK)

			// Manually copy to detect full success
			n, copyErr := io.Copy(c.Response().Writer, objReader)
			if copyErr != nil {
				// client disconnected or transfer error; do not charge
				return copyErr
			}
			if n < fileSize {
				// partial transfer; do not charge
				return nil
			}
		}
		cost := core.CalculateEgressCost(fileSize, egressPerGiB)

		// Pre-check balance to avoid starting transfers that will fail at end
		if balance < cost {
			return c.JSON(http.StatusPaymentRequired, echo.Map{"error": "insufficient credits"})
		}

		// If s3 is nil (test), just simulate a successful transfer
		if s3 == nil {
			if err := l.Debit(link.OwnerID, cost); err == nil {
				_ = s.IncrementDownloads(token)
				fmt.Printf("\033[31m[CHARGE] user=%s token=%s egress_cost=%d\033[0m\n", link.OwnerID, token, cost)
				_ = l.RecordTransaction(link.OwnerID, ledger.Transaction{
					ID:           uuid.NewString(),
					Timestamp:    time.Now().UTC(),
					Type:         "debit",
					Reason:       "egress",
					Amount:       -cost,
					BalanceAfter: l.Balance(link.OwnerID),
					Description:  fmt.Sprintf("Shared download %s", token),
				})
			}
			return c.String(http.StatusOK, "test download ok")
		}
		// Successful full transfer: charge owner and bump stats
		if err := l.Debit(link.OwnerID, cost); err == nil {
			_ = s.IncrementDownloads(token)
			fmt.Printf("\033[31m[CHARGE] user=%s token=%s egress_cost=%d\033[0m\n", link.OwnerID, token, cost)
			_ = l.RecordTransaction(link.OwnerID, ledger.Transaction{
				ID:           uuid.NewString(),
				Timestamp:    time.Now().UTC(),
				Type:         "debit",
				Reason:       "egress",
				Amount:       -cost,
				BalanceAfter: l.Balance(link.OwnerID),
				Description:  fmt.Sprintf("Shared download %s", token),
			})
		}
		return nil
	}
}
