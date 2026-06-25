package minioclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps MinIO with upload and public URL helpers.
type Client struct {
	mc        *minio.Client
	bucket    string
	publicURL string
}

// New connects to MinIO, creates the bucket with public read policy if needed.
func New(endpoint, accessKey, secretKey, bucket, publicURL string) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket check: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
		if err := mc.SetBucketPolicy(ctx, bucket, policy); err != nil {
			return nil, fmt.Errorf("set bucket policy: %w", err)
		}
	}
	return &Client{mc: mc, bucket: bucket, publicURL: publicURL}, nil
}

// Upload stores PNG bytes under the given key and returns the public URL.
func (c *Client) Upload(ctx context.Context, key string, data []byte) (string, error) {
	_, err := c.mc.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "image/png",
	})
	if err != nil {
		return "", fmt.Errorf("put object %s: %w", key, err)
	}
	return fmt.Sprintf("%s/%s/%s", c.publicURL, c.bucket, key), nil
}

// GetOrFetchAsset returns asset bytes from MinIO cache; downloads from sourceURL on cache miss.
// Key example: "assets/heroes/12.png", "assets/avatars/136201811.png"
func (c *Client) GetOrFetchAsset(ctx context.Context, key, sourceURL string) ([]byte, error) {
	// Try cache first
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err == nil {
		data, readErr := io.ReadAll(obj)
		obj.Close()
		if readErr == nil && len(data) > 0 {
			return data, nil
		}
	}

	// Download from source
	resp, err := http.Get(sourceURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download %s: status %d", sourceURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Store in cache (best-effort)
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	_, _ = c.mc.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: ct})

	return data, nil
}

// GetCached returns bytes from MinIO if the key exists, otherwise an error.
// Unlike GetOrFetchAsset it never makes an outbound HTTP download.
func (c *Client) GetCached(ctx context.Context, key string) ([]byte, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(obj)
	obj.Close()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty object")
	}
	return data, nil
}

// CleanOldObjects deletes objects under prefix older than maxAge.
func (c *Client) CleanOldObjects(ctx context.Context, prefix string, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	deleted := 0
	for obj := range c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return deleted, fmt.Errorf("list objects: %w", obj.Err)
		}
		if obj.LastModified.Before(cutoff) {
			if err := c.mc.RemoveObject(ctx, c.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				return deleted, fmt.Errorf("remove %s: %w", obj.Key, err)
			}
			deleted++
		}
	}
	return deleted, nil
}
