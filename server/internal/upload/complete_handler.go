package upload

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

// CompleteUploadRequest represents a request to mark an upload as complete
type CompleteUploadRequest struct {
	// Empty for now - just marking as complete
}

// CompleteUploadResponse represents the response after marking upload complete
type CompleteUploadResponse struct {
	Message  string `json:"message"`
	UploadID string `json:"uploadId"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// HandleCompleteUpload handles POST /files/{uploadId}/complete
// Marks an upload as complete in Redis and verifies S3 object exists
func HandleCompleteUpload(s3Client *storage.S3Client, idxClient storage.Indexer) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get authenticated user
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}

		uploadID := c.Param("uploadId")
		if uploadID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing uploadId parameter"})
		}

		// Compute user hash for S3 key
		userHash := storage.UserHashFromID(userID)

		// Verify S3 object exists
		exists, err := s3Client.ObjectExists(context.Background(), userHash, uploadID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to check S3 object"})
		}
		if !exists {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "S3 object not found - upload may have failed"})
		}

		// Update metadata in Redis to mark as complete
		if idxClient != nil {
			metadata := map[string]string{
				"status": "complete",
			}
			_ = idxClient.StoreFileMetadata(context.Background(), uploadID, metadata)
		}

		return c.JSON(http.StatusOK, CompleteUploadResponse{
			Message:  "upload complete",
			UploadID: uploadID,
		})
	}
}
