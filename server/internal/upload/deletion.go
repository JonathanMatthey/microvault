package upload

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

// DeletionHandler handles file deletion requests for S3-backed storage.
type DeletionHandler struct {
	s3Client *storage.S3Client
	idx      storage.Indexer
	logger   *slog.Logger
}

// NewDeletionHandler creates a new deletion handler for S3.
func NewDeletionHandler(s3Client *storage.S3Client, idx storage.Indexer, logger *slog.Logger) *DeletionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeletionHandler{
		s3Client: s3Client,
		idx:      idx,
		logger:   logger,
	}
}

// Delete removes a file from S3 storage and Redis index.
func (h *DeletionHandler) Delete(c echo.Context) error {
	userID := c.Get("userID").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing user ID"})
	}

	uploadID := c.Param("uploadID")
	if uploadID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing uploadID"})
	}

	// Compute user hash for S3 key isolation
	userHash := storage.UserHashFromID(userID)

	h.logger.Info("delete attempt",
		"user_id", userID,
		"upload_id", uploadID)

	// Verify S3 object exists before deletion
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

	// Delete from S3
	if err := h.s3Client.DeleteObject(context.Background(), userHash, uploadID); err != nil {
		h.logger.Error("failed to delete S3 object",
			"user_id", userID,
			"upload_id", uploadID,
			"error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete file"})
	}

	// Remove from Redis index (best-effort)
	if h.idx != nil {
		metadata := map[string]string{"deleted": "true"}
		if err := h.idx.StoreFileMetadata(context.Background(), uploadID, metadata); err != nil {
			h.logger.Warn("failed to update index",
				"user_id", userID,
				"upload_id", uploadID,
				"error", err)
		}
	}

	h.logger.Info("deletion complete",
		"user_id", userID,
		"upload_id", uploadID)

	return c.JSON(http.StatusOK, map[string]string{
		"message":  "file deleted",
		"uploadId": uploadID,
	})
}
