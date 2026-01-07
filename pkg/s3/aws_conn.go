package s3util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type clientOptions struct {
	endpoint string
}

// Option 用于定制化客户端。
type Option func(*clientOptions)

// WithEndpoint 自定义 S3 兼容端点（如本地或私有云）。
func WithEndpoint(endpoint string) Option {
	return func(o *clientOptions) {
		o.endpoint = strings.TrimSpace(endpoint)
	}
}

// Client 封装 S3 客户端与默认 Bucket。
type Client struct {
	s3     *s3.Client
	bucket string
}

// NewClient 创建可复用的 S3 客户端。
// region 为空则使用环境或配置文件中的默认区域。
func NewClient(ctx context.Context, bucket, region string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("bucket name is required")
	}

	var copt clientOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&copt)
		}
	}

	loadOpts := []func(*config.LoadOptions) error{}
	if region != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	newOpts := func(o *s3.Options) {
		if copt.endpoint != "" {
			o.BaseEndpoint = aws.String(copt.endpoint)
		}
	}

	return &Client{
		s3:     s3.NewFromConfig(cfg, newOpts),
		bucket: bucket,
	}, nil
}

// Upload 上传任意 Reader 内容到指定 Key（覆盖同名对象）。
func (c *Client) Upload(ctx context.Context, key string, body io.Reader) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// UploadFile 读取本地文件并上传。
func (c *Client) UploadFile(ctx context.Context, key, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer f.Close()
	return c.Upload(ctx, key, f)
}

// Download 下载指定 Key 内容，返回字节切片。
func (c *Client) Download(ctx context.Context, key string) ([]byte, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}

	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read object %s: %w", key, err)
	}
	return data, nil
}

// Update 用新内容覆盖已有对象（本质等同 Upload）。
func (c *Client) Update(ctx context.Context, key string, body io.Reader) error {
	return c.Upload(ctx, key, body)
}

// UpdateFile 用本地文件覆盖已有对象。
func (c *Client) UpdateFile(ctx context.Context, key, filePath string) error {
	return c.UploadFile(ctx, key, filePath)
}

// normalizeKey 清理 Key，避免空字符串或越级路径。
func normalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "/")
	key = path.Clean(key)
	if key == "." || key == "/" || key == "" || strings.Contains(key, "..") {
		return "", errors.New("invalid s3 object key")
	}
	return key, nil
}
