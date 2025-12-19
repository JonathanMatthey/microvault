package upload

import (
	"context"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/core"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

// FileListItem represents a file in the user's storage
type FileListItem struct {
	ID                    string `json:"id"`
	UploadID              string `json:"uploadId"`
	Filename              string `json:"filename"`
	Size                  int64  `json:"size"`
	UploadedAt            string `json:"uploadedAt"`
	LastModified          string `json:"lastModified"`
	EstimatedDownloadCost int64  `json:"estimatedDownloadCost"`
	DownloadURL           string `json:"downloadUrl,omitempty"`
	UploadCost            int64  `json:"uploadCost"`
	DownloadCost          int64  `json:"downloadCost"`
	StorageMonthlyCost    int64  `json:"storageMonthlyCost"`
}

// HandleListFiles handles GET /files
// Lists all files for the authenticated user from S3
func HandleListFiles(s3Client *storage.S3Client, ingressPerGiB int64, egressPerGiB int64, storagePerGiBMonth int64) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}

		// Compute user hash
		userHash := storage.UserHashFromID(userID)

		log.Printf("ListFiles: listing files for user=%s, hash=%s", userID, userHash)

		// List files from S3
		files, err := s3Client.ListUserFiles(context.Background(), userHash)
		if err != nil {
			log.Printf("ListFiles: failed to list files for user=%s: %v", userID, err)
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to list files"})
		}

		log.Printf("ListFiles: found %d files for user=%s", len(files), userID)

		// Convert to response format
		response := make([]FileListItem, 0, len(files))
		ctx := c.Request().Context()
		for _, f := range files {
			uploadCost := core.CalculateIngressCost(f.Size, ingressPerGiB)
			downloadCost := core.CalculateEgressCost(f.Size, egressPerGiB)
			storageMonthlyCost := core.CalculateStorageMonthlyCost(f.Size, storagePerGiBMonth)

			filename := f.Filename
			if filename == "" {
				filename = f.UploadID
			}
			if meta, err := s3Client.GetObjectMetadata(ctx, f.Key); err == nil {
				if fname, ok := meta["filename"]; ok && fname != "" {
					filename = fname
				}
			}

			response = append(response, FileListItem{
				ID:                    f.UploadID,
				UploadID:              f.UploadID,
				Filename:              filename,
				Size:                  f.Size,
				UploadedAt:            f.LastModified.Format("2006-01-02T15:04:05Z"),
				LastModified:          f.LastModified.Format("2006-01-02T15:04:05Z"),
				EstimatedDownloadCost: downloadCost,
				UploadCost:            uploadCost,
				DownloadCost:          downloadCost,
				StorageMonthlyCost:    storageMonthlyCost,
			})
		}

		return c.JSON(http.StatusOK, response)
	}
}
