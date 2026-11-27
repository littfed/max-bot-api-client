package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultS3Timeout = 15 * time.Second

// ErrObjectNotFound is returned when a requested object key does not exist.
var ErrObjectNotFound = errors.New("storage: object not found")

// ObjectStorage defines an abstraction over binary object storage (S3/MinIO or in-memory).
type ObjectStorage interface {
	PutObject(ctx context.Context, key, contentType string, data []byte) (string, error)
	GetObject(ctx context.Context, key string) ([]byte, string, error)
	DeleteObject(ctx context.Context, key string) error
}

// S3Config configures an S3/MinIO-compatible ObjectStorage client.
type S3Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	PublicURL string
	UseSSL    bool
}

type memoryObject struct {
	contentType string
	data        []byte
}

// MemoryStorage is a thread-safe in-memory ObjectStorage for local development and tests.
type MemoryStorage struct {
	mu            sync.RWMutex
	publicBaseURL string
	objects       map[string]memoryObject
}

// NewMemoryStorage creates a MemoryStorage that constructs public URLs using publicBaseURL.
func NewMemoryStorage(publicBaseURL string) *MemoryStorage {
	return &MemoryStorage{
		publicBaseURL: strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
		objects:       make(map[string]memoryObject),
	}
}

// PutObject stores data under key and returns its normalized public URL.
func (m *MemoryStorage) PutObject(ctx context.Context, key, contentType string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normKey, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	copied := make([]byte, len(data))
	copy(copied, data)

	m.mu.Lock()
	m.objects[normKey] = memoryObject{
		contentType: contentType,
		data:        copied,
	}
	m.mu.Unlock()

	return joinPublicURL(m.publicBaseURL, normKey), nil
}

// GetObject retrieves a copy of the stored object bytes and content type for key.
func (m *MemoryStorage) GetObject(ctx context.Context, key string) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	normKey, err := normalizeKey(key)
	if err != nil {
		return nil, "", err
	}

	m.mu.RLock()
	obj, ok := m.objects[normKey]
	m.mu.RUnlock()
	if !ok {
		return nil, "", ErrObjectNotFound
	}

	copied := make([]byte, len(obj.data))
	copy(copied, obj.data)
	return copied, obj.contentType, nil
}

// DeleteObject removes key from memory storage.
func (m *MemoryStorage) DeleteObject(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	normKey, err := normalizeKey(key)
	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.objects, normKey)
	m.mu.Unlock()
	return nil
}

// S3Storage stores objects in an S3/MinIO-compatible bucket.
type S3Storage struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

// NewS3Storage initializes a MinIO/S3 client and ensures the target bucket exists.
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	endpoint, useSSL := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if endpoint == "" {
		return nil, errors.New("storage: S3 endpoint is required")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, errors.New("storage: S3 bucket is required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: init s3 client: %w", err)
	}

	opCtx, cancel := withDefaultTimeout(ctx, defaultS3Timeout)
	defer cancel()

	exists, err := client.BucketExists(opCtx, bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: check bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(opCtx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("storage: create bucket %q: %w", bucket, err)
		}
	}

	publicURL := strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	if publicURL == "" {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		publicURL = fmt.Sprintf("%s://%s/%s", scheme, endpoint, bucket)
	}

	return &S3Storage{
		client:    client,
		bucket:    bucket,
		publicURL: publicURL,
	}, nil
}

// PutObject uploads data to S3 and returns its normalized public URL.
func (s *S3Storage) PutObject(ctx context.Context, key, contentType string, data []byte) (string, error) {
	normKey, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	opCtx, cancel := withDefaultTimeout(ctx, defaultS3Timeout)
	defer cancel()

	_, err = s.client.PutObject(
		opCtx,
		s.bucket,
		normKey,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", fmt.Errorf("storage: put object %q: %w", normKey, err)
	}

	return joinPublicURL(s.publicURL, normKey), nil
}

// GetObject downloads an object from S3 and returns its bytes and content type.
func (s *S3Storage) GetObject(ctx context.Context, key string) ([]byte, string, error) {
	normKey, err := normalizeKey(key)
	if err != nil {
		return nil, "", err
	}

	opCtx, cancel := withDefaultTimeout(ctx, defaultS3Timeout)
	defer cancel()

	obj, err := s.client.GetObject(opCtx, s.bucket, normKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", mapS3Error("get", normKey, err)
	}
	defer func() { _ = obj.Close() }()

	info, err := obj.Stat()
	if err != nil {
		return nil, "", mapS3Error("stat", normKey, err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", mapS3Error("read", normKey, err)
	}

	return data, info.ContentType, nil
}

// DeleteObject removes an object from S3.
func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	normKey, err := normalizeKey(key)
	if err != nil {
		return err
	}

	opCtx, cancel := withDefaultTimeout(ctx, defaultS3Timeout)
	defer cancel()

	if err := s.client.RemoveObject(opCtx, s.bucket, normKey, minio.RemoveObjectOptions{}); err != nil {
		return mapS3Error("delete", normKey, err)
	}
	return nil
}

// NewObjectStorageFromEnv creates S3Storage from S3_* environment variables or falls back to MemoryStorage when S3_ENDPOINT is empty.
func NewObjectStorageFromEnv(ctx context.Context) (ObjectStorage, error) {
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	if endpoint == "" {
		return NewMemoryStorage("/api/v1/images/assets"), nil
	}

	var useSSL bool
	if rawSSL := strings.TrimSpace(os.Getenv("S3_USE_SSL")); rawSSL != "" {
		parsed, err := strconv.ParseBool(rawSSL)
		if err != nil {
			return nil, fmt.Errorf("storage: invalid S3_USE_SSL %q: %w", rawSSL, err)
		}
		useSSL = parsed
	}

	return NewS3Storage(ctx, S3Config{
		Endpoint:  endpoint,
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		PublicURL: os.Getenv("S3_PUBLIC_URL"),
		UseSSL:    useSSL,
	})
}

func normalizeKey(key string) (string, error) {
	norm := strings.TrimLeft(strings.TrimSpace(key), "/")
	if norm == "" {
		return "", errors.New("storage: object key cannot be empty")
	}
	return norm, nil
}

func joinPublicURL(baseURL, normKey string) string {
	if baseURL == "" {
		return "/" + normKey
	}
	return baseURL + "/" + normKey
}

func normalizeEndpoint(raw string, useSSL bool) (string, bool) {
	endpoint := strings.TrimSpace(raw)
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useSSL = true
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	return strings.TrimRight(endpoint, "/"), useSSL
}

func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func mapS3Error(op, key string, err error) error {
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrObjectNotFound, key)
	}
	return fmt.Errorf("storage: %s object %q: %w", op, key, err)
}
