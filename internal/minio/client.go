package minioclient

import (
	"bytes"
	"context"
	"fmt"

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
