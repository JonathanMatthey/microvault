package upload

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
)

// HandleProxyUpload handles POST /files/upload for direct file upload through backend
// This endpoint proxies the file upload to S3, avoiding CORS issues in development
func HandleProxyUpload(s3Client *storage.S3Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Get("userID").(string)
		if userID == "" {
			log.Printf("ProxyUpload: missing userID")
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}

		uploadID := c.Param("uploadId")
		if uploadID == "" {
			log.Printf("ProxyUpload: missing uploadId")
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing uploadId"})
		}

		log.Printf("ProxyUpload: starting upload for user=%s, uploadId=%s", userID, uploadID)

		// Get filename from header if provided
		filename := c.Request().Header.Get("X-Filename")
		if filename == "" {
			filename = uploadID // Fallback to uploadID
		}
		log.Printf("ProxyUpload: filename=%s", filename)

		// Read file from request body
		file, err := c.FormFile("file")
		if err != nil {
			log.Printf("ProxyUpload: no form file, streaming raw body for uploadId=%s", uploadID)

			// Compute user hash for S3 key using real filename
			userHash := storage.UserHashFromID(userID)
			key := storage.BuildObjectKey(userHash, uploadID, filename)

			log.Printf("ProxyUpload: uploading to S3 key=%s (streaming, content-length=%d)", key, c.Request().ContentLength)

			// Upload to S3 with metadata (filename stored redundantly)
			var cl *int64
			if c.Request().ContentLength > 0 {
				v := c.Request().ContentLength
				cl = &v
			}
			size, err := s3Client.PutObjectWithMetadataStream(context.Background(), key, c.Request().Body, map[string]string{"filename": filename}, cl)
			if err != nil {
				log.Printf("ProxyUpload: S3 upload failed: %v", err)
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to upload to S3"})
			}

			log.Printf("ProxyUpload: S3 upload successful for uploadId=%s", uploadID)
			return c.JSON(http.StatusOK, echo.Map{
				"uploadId": uploadID,
				"status":   "uploaded",
				"size":     size,
			})
		}

		// Form file upload
		log.Printf("ProxyUpload: using form file for uploadId=%s", uploadID)
		src, err := file.Open()
		if err != nil {
			log.Printf("ProxyUpload: failed to open form file: %v", err)
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "failed to open file"})
		}
		defer src.Close()

		// Compute user hash for S3 key using real filename
		userHash := storage.UserHashFromID(userID)
		key := storage.BuildObjectKey(userHash, uploadID, filename)

		log.Printf("ProxyUpload: uploading to S3 key=%s (streaming form, size=%d)", key, file.Size)

		// Upload to S3 with request context and metadata
		ctx := c.Request().Context()
		metadata := map[string]string{
			"filename": filename,
		}
		var cl *int64
		if file.Size > 0 {
			v := file.Size
			cl = &v
		}
		size, err := s3Client.PutObjectWithMetadataStream(ctx, key, src, metadata, cl)
		if err != nil {
			log.Printf("ProxyUpload: S3 upload failed for uploadId=%s: %v", uploadID, err)
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to upload to S3"})
		}

		log.Printf("ProxyUpload: S3 upload successful for uploadId=%s", uploadID)
		return c.JSON(http.StatusOK, echo.Map{
			"uploadId": uploadID,
			"status":   "uploaded",
			"size":     size,
		})
	}
}

// SimplifiedUploadFlow handles POST /files/upload-simple for a simpler upload flow
// Takes file directly and returns uploadId
func SimplifiedUploadFlow(ledgerSvc ledger.Ledger, s3Client *storage.S3Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Get("userID").(string)
		if userID == "" {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing user ID"})
		}

		// Read raw body
		fileContent, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "failed to read file"})
		}

		// Generate upload ID (length-based placeholder) and key using filename when provided
		uploadID := fmt.Sprintf("%d", len(fileContent))
		filename := c.Request().Header.Get("X-Filename")
		if filename == "" {
			filename = uploadID
		}

		// Compute user hash and upload key
		userHash := storage.UserHashFromID(userID)
		key := storage.BuildObjectKey(userHash, uploadID, filename)

		// Upload to S3
		err = s3Client.PutObject(context.Background(), key, fileContent)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to upload to S3"})
		}

		return c.JSON(http.StatusOK, echo.Map{
			"uploadId": uploadID,
			"filename": filename,
		})
	}
}
