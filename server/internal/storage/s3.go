package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Indexer interface for metadata storage (Redis-backed)
type Indexer interface {
	StoreFileMetadata(ctx context.Context, uploadID string, metadata map[string]string) error
}

// S3Client wraps AWS S3 client and provides presigned URL generation and S3 operations
type S3Client struct {
	client *s3.Client
	bucket string
	cfg    S3Config
}

// S3Config holds S3 connection configuration
type S3Config struct {
	Endpoint                 string
	Region                   string
	Bucket                   string
	AccessKey                string
	SecretKey                string
	PresignedURLExpiryUpload int // seconds
	PresignedURLExpiryDL     int // seconds
}

// NewS3Client creates a new S3 client from configuration
func NewS3Client(cfg S3Config) (*S3Client, error) {
	// Create AWS credentials from static keys
	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKey,
		cfg.SecretKey,
		"", // no session token for basic auth
	)

	// Create custom resolver to use Hetzner endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           cfg.Endpoint,
			SigningRegion: cfg.Region,
		}, nil
	})

	// Create AWS SDK config
	awsCfg := aws.Config{
		Region:                      cfg.Region,
		Credentials:                 creds,
		EndpointResolverWithOptions: customResolver,
	}

	// Create S3 client with path-style addressing for S3-compatible services
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Client{
		client: client,
		bucket: cfg.Bucket,
		cfg:    cfg,
	}, nil
}

// Config returns the S3 configuration
func (sc *S3Client) Config() S3Config {
	return sc.cfg
}

// sanitizeFilename ensures we do not leak path separators into S3 object keys.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" {
		return "file"
	}
	return name
}

// BuildObjectKey creates the canonical S3 key for a user's upload using the real filename.
// Format: users/{userHash}/{uploadId}-{filename}
func BuildObjectKey(userHash, uploadID, filename string) string {
	return fmt.Sprintf("users/%s/%s-%s", userHash, uploadID, sanitizeFilename(filename))
}

// ParseObjectKey extracts uploadId and filename from a canonical key.
// For UUID upload IDs (36 chars, with hyphens), we slice the first 36 chars to avoid ambiguity with filename hyphens.
func ParseObjectKey(key, userHash string) (uploadID, filename string, ok bool) {
	prefix := fmt.Sprintf("users/%s/", userHash)
	if !strings.HasPrefix(key, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	const uuidLen = 36
	if len(rest) > uuidLen && rest[uuidLen] == '-' { // uuid + '-' + filename
		uploadID = rest[:uuidLen]
		filename = rest[uuidLen+1:]
		return uploadID, filename, true
	}
	// Fallback: split on first hyphen
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// FindObjectByUploadID returns the first object key (and metadata) matching an uploadId prefix.
func (sc *S3Client) FindObjectByUploadID(ctx context.Context, userHash, uploadID string) (FileInfo, error) {
	prefix := fmt.Sprintf("users/%s/%s", userHash, uploadID)
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(sc.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	}

	result, err := sc.client.ListObjectsV2(ctx, input)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to find object: %w", err)
	}
	if len(result.Contents) == 0 || result.Contents[0].Key == nil {
		return FileInfo{}, fmt.Errorf("object not found for uploadId=%s", uploadID)
	}

	obj := result.Contents[0]
	key := aws.ToString(obj.Key)

	// Attempt to read metadata (best effort)
	var metadata map[string]string
	if head, err := sc.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(key),
	}); err == nil && head.Metadata != nil {
		metadata = head.Metadata
	}

	uploadIDParsed, filenameParsed, ok := ParseObjectKey(key, userHash)
	if !ok {
		uploadIDParsed = uploadID
		filenameParsed = key
	}

	return FileInfo{
		UploadID:     uploadIDParsed,
		Filename:     filenameParsed,
		Key:          key,
		Size:         aws.ToInt64(obj.Size),
		LastModified: aws.ToTime(obj.LastModified),
		Metadata:     metadata,
	}, nil
}

// GeneratePresignedPutURL generates a presigned URL for direct client upload to S3 using the real filename.
func (sc *S3Client) GeneratePresignedPutURL(ctx context.Context, userHash string, uploadID string, filename string) (string, error) {
	key := BuildObjectKey(userHash, uploadID, filename)

	presigner := s3.NewPresignClient(sc.client)

	putRequest, err := presigner.PresignPutObject(ctx,
		&s3.PutObjectInput{
			Bucket: aws.String(sc.bucket),
			Key:    aws.String(key),
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(sc.cfg.PresignedURLExpiryUpload) * time.Second
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT URL: %w", err)
	}

	return putRequest.URL, nil
}

// GeneratePresignedGetURL generates a presigned URL for direct client download from S3 by locating the object key.
func (sc *S3Client) GeneratePresignedGetURL(ctx context.Context, userHash string, uploadID string) (string, error) {
	info, err := sc.FindObjectByUploadID(ctx, userHash, uploadID)
	if err != nil {
		return "", err
	}
	key := info.Key

	presigner := s3.NewPresignClient(sc.client)

	getRequest, err := presigner.PresignGetObject(ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(sc.bucket),
			Key:    aws.String(key),
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(sc.cfg.PresignedURLExpiryDL) * time.Second
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	return getRequest.URL, nil
}

// ObjectExists checks if an object exists in S3 for an uploadId prefix
func (sc *S3Client) ObjectExists(ctx context.Context, userHash string, uploadID string) (bool, error) {
	_, err := sc.FindObjectByUploadID(ctx, userHash, uploadID)
	if err != nil {
		if strings.Contains(err.Error(), "object not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ObjectMetadata contains HTTP headers for streaming downloads
type ObjectMetadata struct {
	ContentType   string
	ContentLength string
	Filename      string
}

// GetObject retrieves an object from S3 and returns a reader with metadata
func (sc *S3Client) GetObject(ctx context.Context, userHash string, uploadID string) (io.ReadCloser, ObjectMetadata, error) {
	info, err := sc.FindObjectByUploadID(ctx, userHash, uploadID)
	if err != nil {
		return nil, ObjectMetadata{}, fmt.Errorf("failed to find object: %w", err)
	}

	result, err := sc.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(info.Key),
	})
	if err != nil {
		return nil, ObjectMetadata{}, fmt.Errorf("failed to get object: %w", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	contentLength := fmt.Sprintf("%d", info.Size)
	if result.ContentLength != nil {
		contentLength = fmt.Sprintf("%d", *result.ContentLength)
	}

	return result.Body, ObjectMetadata{
		ContentType:   contentType,
		ContentLength: contentLength,
		Filename:      info.Filename,
	}, nil
}

// PutObject uploads content to S3 directly (used for backend-proxied uploads)
func (sc *S3Client) PutObject(ctx context.Context, key string, content []byte) error {
	fmt.Printf("S3Client.PutObject: starting upload, key=%s, size=%d bytes\n", key, len(content))

	_, err := sc.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	})

	if err != nil {
		fmt.Printf("S3Client.PutObject: FAILED for key=%s, error=%v\n", key, err)
		return fmt.Errorf("failed to put object: %w", err)
	}

	fmt.Printf("S3Client.PutObject: SUCCESS for key=%s\n", key)
	return nil
}

// PutObjectWithMetadataStream uploads content to S3 with metadata using streaming and returns bytes sent.
func (sc *S3Client) PutObjectWithMetadataStream(ctx context.Context, key string, reader io.Reader, metadata map[string]string, contentLength *int64) (int64, error) {
	count := &countWriter{}
	body := io.TeeReader(reader, count)

	fmt.Printf("S3Client.PutObjectWithMetadata: starting upload, key=%s, metadata=%v\n", key, metadata)

	input := &s3.PutObjectInput{
		Bucket:   aws.String(sc.bucket),
		Key:      aws.String(key),
		Body:     body,
		Metadata: metadata,
	}
	if contentLength != nil {
		input.ContentLength = contentLength
	}

	_, err := sc.client.PutObject(ctx, input)

	if err != nil {
		fmt.Printf("S3Client.PutObjectWithMetadata: FAILED for key=%s, error=%v\n", key, err)
		return count.n, fmt.Errorf("failed to put object: %w", err)
	}

	fmt.Printf("S3Client.PutObjectWithMetadata: SUCCESS for key=%s, bytes=%d\n", key, count.n)
	return count.n, nil
}

type countWriter struct{ n int64 }

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

// GetObjectMetadata retrieves metadata for an S3 object using its full key
func (sc *S3Client) GetObjectMetadata(ctx context.Context, key string) (map[string]string, error) {
	resp, err := sc.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	return resp.Metadata, nil
}

// DeleteObject deletes an object from S3
func (sc *S3Client) DeleteObject(ctx context.Context, userHash string, uploadID string) error {
	info, err := sc.FindObjectByUploadID(ctx, userHash, uploadID)
	if err != nil {
		return err
	}

	_, err = sc.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(info.Key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// FileInfo represents metadata about a file in S3
type FileInfo struct {
	UploadID     string
	Filename     string
	Key          string
	Size         int64
	LastModified time.Time
	Metadata     map[string]string
}

// ListUserFiles lists all files for a specific user
func (sc *S3Client) ListUserFiles(ctx context.Context, userHash string) ([]FileInfo, error) {
	prefix := fmt.Sprintf("users/%s/", userHash)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(sc.bucket),
		Prefix: aws.String(prefix),
	}

	result, err := sc.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var files []FileInfo
	for _, obj := range result.Contents {
		if obj.Key == nil {
			continue
		}

		key := aws.ToString(obj.Key)
		uploadID, filename, ok := ParseObjectKey(key, userHash)
		if !ok {
			continue
		}

		files = append(files, FileInfo{
			UploadID:     uploadID,
			Filename:     filename,
			Key:          key,
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}

	return files, nil
}

// UserHashFromID computes SHA256 hash of user ID
func UserHashFromID(userID string) string {
	h := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("%x", h)
}
