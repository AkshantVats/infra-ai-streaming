// SPDX-License-Identifier: MIT
package objectstore

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStore is an ObjectStore backed by a MinIO (or any S3-compatible)
// bucket. It is used in production when a MinIO endpoint is configured; it
// is not exercised by unit tests since no live service runs in CI, but
// go build/go vet still cover it.
type MinioStore struct {
	client *minio.Client
	bucket string
}

// MinioConfig holds the connection parameters for NewMinioStore.
type MinioConfig struct {
	Endpoint        string // host:port, no scheme
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

// NewMinioStore connects to a MinIO endpoint and returns an ObjectStore
// backed by cfg.Bucket. It does not verify the bucket exists — callers that
// need that guarantee should call BucketExists themselves before use.
func NewMinioStore(cfg MinioConfig) (*MinioStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinioStore{client: client, bucket: cfg.Bucket}, nil
}

// BucketExists reports whether the configured bucket exists.
func (s *MinioStore) BucketExists(ctx context.Context) (bool, error) {
	return s.client.BucketExists(ctx, s.bucket)
}

// Put writes body (exactly size bytes) to key, replacing any existing
// object at that key.
func (s *MinioStore) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

// Get returns a reader over the object stored at key. The caller must
// Close it. Returns ErrNotFound if key does not exist.
func (s *MinioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translateMinioErr(err)
	}
	// GetObject is lazy: the request isn't actually issued until the first
	// read, so a missing key surfaces on Stat here rather than on Get above.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, translateMinioErr(err)
	}
	return obj, nil
}

// Delete removes the object at key. Returns ErrNotFound if key does not
// exist.
func (s *MinioStore) Delete(ctx context.Context, key string) error {
	if _, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err != nil {
		return translateMinioErr(err)
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// List returns metadata for every object whose key starts with prefix.
func (s *MinioStore) List(ctx context.Context, prefix string) ([]ObjectMeta, error) {
	var out []ObjectMeta
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, ObjectMeta{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		})
	}
	return out, nil
}

// translateMinioErr maps a "key does not exist" MinIO error response to the
// package-level ErrNotFound so callers can use errors.Is uniformly across
// both ObjectStore implementations.
func translateMinioErr(err error) error {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && (resp.Code == "NoSuchKey" || resp.Code == "NoSuchObject") {
		return ErrNotFound
	}
	return err
}
