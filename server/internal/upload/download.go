package upload

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/core"
	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/policy"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

// DownloadHandler handles file download requests using S3 presigned URLs.
type DownloadHandler struct {
	ledger       ledger.Ledger
	policy       policy.Engine
	s3Client     *storage.S3Client
	logger       *slog.Logger
	egressPerGiB int64
}

// NewDownloadHandler creates a new download handler for S3.
func NewDownloadHandler(ledger ledger.Ledger, policy policy.Engine, s3Client *storage.S3Client, logger *slog.Logger, egressPerGiB int64) *DownloadHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DownloadHandler{
		ledger:       ledger,
		policy:       policy,
		s3Client:     s3Client,
		logger:       logger,
		egressPerGiB: egressPerGiB,
	}
}

// Download generates a presigned S3 GET URL and redirects the client.
// Credits are deducted upfront for the full file cost.
func (h *DownloadHandler) Download(c echo.Context) error {
	userID := c.Get("userID").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing user ID"})
	}

	uploadID := c.Param("uploadID")
	if uploadID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing uploadID"})
	}

	// Check if user is frozen
	balance := h.ledger.Balance(userID)
	if !h.policy.CanDownload(balance) {
		h.logger.Warn("download blocked: account frozen",
			"user_id", userID,
			"balance", balance)
		return c.JSON(http.StatusPaymentRequired, map[string]string{
			"error": "account frozen",
		})
	}

	// Compute user hash for S3 key isolation
	userHash := storage.UserHashFromID(userID)

	// Verify S3 object exists and get metadata
	exists, err := h.s3Client.ObjectExists(context.Background(), userHash, uploadID)
	if err != nil {
		h.logger.Error("failed to check S3 object",
			"user_id", userID,
			"upload_id", uploadID,
			"error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check file"})
	}
	if !exists {
		h.logger.Warn("file not found in S3",
			"user_id", userID,
			"upload_id", uploadID)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}

	// Get object metadata to determine actual file size
	objReader, objMetadata, err := h.s3Client.GetObject(context.Background(), userHash, uploadID)
	if err != nil {
		h.logger.Error("failed to get object from S3",
			"user_id", userID,
			"upload_id", uploadID,
			"error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to retrieve file"})
	}
	defer objReader.Close()

	// Parse file size from metadata
	var fileSize int64
	if objMetadata.ContentLength != "" {
		fmt.Sscanf(objMetadata.ContentLength, "%d", &fileSize)
	}
	cost := core.CalculateEgressCost(fileSize, h.egressPerGiB)

	// Check balance
	if balance < cost {
		h.logger.Warn("insufficient balance for download",
			"user_id", userID,
			"balance", balance,
			"required", cost)
		return c.JSON(http.StatusPaymentRequired, map[string]string{"error": "insufficient credits"})
	}

	// Deduct credits upfront
	if err := h.ledger.Debit(userID, cost); err != nil {
		h.logger.Error("failed to debit credits",
			"user_id", userID,
			"cost", cost,
			"error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to process download"})
	} else {
		// Red log for charge event
		fmt.Printf("\033[31m[CHARGE] user=%s upload_id=%s egress_cost=%d\033[0m\n", userID, uploadID, cost)
		_ = h.ledger.RecordTransaction(userID, ledger.Transaction{
			ID:           uuid.NewString(),
			Timestamp:    time.Now().UTC(),
			Type:         "debit",
			Reason:       "egress",
			Amount:       -cost,
			BalanceAfter: h.ledger.Balance(userID),
			Description:  fmt.Sprintf("Egress charge for %s (%d bytes)", uploadID, fileSize),
		})
	}

	// Stream the file directly from S3 through our backend (reuse objReader from above)
	// Set response headers
	c.Response().Header().Set("Content-Type", objMetadata.ContentType)
	c.Response().Header().Set("Content-Length", objMetadata.ContentLength)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", objMetadata.Filename))
	c.Response().Header().Set("Cache-Control", "private, max-age=0")

	h.logger.Info("download initiated",
		"user_id", userID,
		"upload_id", uploadID,
		"cost", cost,
		"size", fileSize)

	// Stream the file to the client
	c.Response().WriteHeader(http.StatusOK)
	return c.Stream(http.StatusOK, objMetadata.ContentType, objReader)
}
