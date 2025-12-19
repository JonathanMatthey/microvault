package upload

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/selfstack/internal/core"
	"github.com/bosbaber/hackweek/selfstack/internal/ledger"
	"github.com/bosbaber/hackweek/selfstack/internal/policy"
	"github.com/bosbaber/hackweek/selfstack/internal/storage"
)

// UploadURLRequest represents a request for a presigned upload URL
type UploadURLRequest struct {
	Filename    string `json:"filename" validate:"required"`
	Size        int64  `json:"size" validate:"required,min=1"`
	ContentType string `json:"contentType" validate:"required"`
}

// UploadURLResponse represents the response with presigned URL
type UploadURLResponse struct {
	UploadID  string `json:"uploadId"`
	UploadURL string `json:"uploadUrl"`
	ExpiresIn int    `json:"expiresIn"`
	Cost      int64  `json:"cost"`
	Filename  string `json:"filename"`
}

// HandleGetUploadURL handles POST /files/upload-url
// Generates a presigned S3 PUT URL for direct client upload
func HandleGetUploadURL(ledgerSvc ledger.Ledger, policySvc policy.Engine, s3Client *storage.S3Client, idxClient storage.Indexer, ingressPerGiB int64) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get authenticated user
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}

		// Parse request
		var req UploadURLRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
		}

		// Get current balance
		balance := ledgerSvc.Balance(userID)

		// Calculate ingress cost
		cost := core.CalculateIngressCost(req.Size, ingressPerGiB)

		// Check policy: can upload?
		canUpload := policySvc.CanUpload(balance)
		if !canUpload {
			return c.JSON(http.StatusPaymentRequired, echo.Map{"error": "account frozen or insufficient credits"})
		}

		// Check sufficient balance
		if balance < cost {
			return c.JSON(http.StatusPaymentRequired, echo.Map{"error": "insufficient credits"})
		}

		// Generate upload ID
		uploadID := uuid.New().String()

		// Compute user hash for S3 key
		userHash := storage.UserHashFromID(userID)

		// Generate presigned PUT URL with real filename in the key
		presignedURL, err := s3Client.GeneratePresignedPutURL(context.Background(), userHash, uploadID, req.Filename)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to generate presigned URL"})
		}

		// Deduct credits upfront
		err = ledgerSvc.Debit(userID, cost)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to deduct credits"})
		} else {
			// Red log for ingress charge
			fmt.Printf("\033[31m[CHARGE] user=%s upload_id=%s ingress_cost=%d\033[0m\n", userID, uploadID, cost)
			_ = ledgerSvc.RecordTransaction(userID, ledger.Transaction{
				ID:           uuid.NewString(),
				Timestamp:    time.Now().UTC(),
				Type:         "debit",
				Reason:       "ingress",
				Amount:       -cost,
				BalanceAfter: ledgerSvc.Balance(userID),
				Description:  fmt.Sprintf("Ingress charge for %s (%d bytes)", req.Filename, req.Size),
			})
		}

		// Store metadata in Redis (optional - for tracking)
		if idxClient != nil {
			// Store in Redis for later retrieval
			metadata := map[string]string{
				"userId":      userID,
				"uploadId":    uploadID,
				"filename":    req.Filename,
				"size":        "",
				"contentType": req.ContentType,
				"createdAt":   core.NowRFC3339(),
				"status":      "pending",
				"s3Key":       storage.BuildObjectKey(userHash, uploadID, req.Filename),
			}
			_ = idxClient.StoreFileMetadata(context.Background(), uploadID, metadata)
		}

		return c.JSON(http.StatusOK, UploadURLResponse{
			UploadID:  uploadID,
			UploadURL: presignedURL,
			ExpiresIn: s3Client.Config().PresignedURLExpiryUpload,
			Cost:      cost,
			Filename:  req.Filename,
		})
	}
}
