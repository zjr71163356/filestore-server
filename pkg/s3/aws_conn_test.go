package s3util

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestNewClientBaseEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		wantBase string
	}{
		{
			name:     "custom endpoint uses BaseEndpoint",
			endpoint: "  http://127.0.0.1:9000  ",
			wantBase: "http://127.0.0.1:9000",
		},
		{
			name:     "empty endpoint keeps default resolver",
			endpoint: "",
			wantBase: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY_ID", "dummy-access")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "dummy-secret")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

			opts := []Option{}
			if tc.endpoint != "" {
				opts = append(opts, WithEndpoint(tc.endpoint))
			}

			c, err := NewClient(context.Background(), "test-bucket", "us-east-1", opts...)
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			base := c.s3.Options().BaseEndpoint
			if tc.wantBase == "" {
				if base != nil {
					t.Fatalf("expected BaseEndpoint nil, got %q", *base)
				}
				return
			}

			if base == nil {
				t.Fatalf("BaseEndpoint nil, want %q", tc.wantBase)
			}
			if *base != tc.wantBase {
				t.Fatalf("BaseEndpoint mismatch: want %q got %q", tc.wantBase, *base)
			}
		})
	}
}

const (
	s3TestBucketEnv   = "S3_TEST_BUCKET"
	s3TestEndpointEnv = "S3_TEST_ENDPOINT"
)

func requireS3Config(t *testing.T) (string, string, string) {
	t.Helper()

	bucket := strings.TrimSpace(os.Getenv(s3TestBucketEnv))
	if bucket == "" {
		t.Skipf("%s not set; skipping real S3 test", s3TestBucketEnv)
	}

	region := strings.TrimSpace(os.Getenv("S3_TEST_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	endpoint := strings.TrimSpace(os.Getenv(s3TestEndpointEnv))
	return bucket, region, endpoint
}

func newRealClient(t *testing.T) (*Client, string) {
	t.Helper()

	bucket, region, endpoint := requireS3Config(t)
	opts := []Option{}
	if endpoint != "" {
		opts = append(opts, WithEndpoint(endpoint))
	}

	client, err := NewClient(context.Background(), bucket, region, opts...)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client, testPrefix(t)
}

func testPrefix(t *testing.T) string {
	t.Helper()

	name := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	return "test/" + name + "/" + fmtTimestamp()
}

func fmtTimestamp() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}

func registerCleanup(t *testing.T, client *Client) func(string) {
	t.Helper()

	keys := make([]string, 0, 4)
	t.Cleanup(func() {
		for _, key := range keys {
			_, err := client.s3.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(client.bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				t.Logf("cleanup delete object %q failed: %v", key, err)
			}
		}
	})
	return func(key string) {
		keys = append(keys, key)
	}
}

func TestClientUploadDownloadUpdate(t *testing.T) {
	client, prefix := newRealClient(t)
	ctx := context.Background()
	cleanup := registerCleanup(t, client)
	key := prefix + "/dir/hello.txt"
	cleanup(key)

	if err := client.Upload(ctx, key, strings.NewReader("hello")); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	got, err := client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("Download mismatch: want %q got %q", "hello", string(got))
	}

	if err := client.Update(ctx, key, strings.NewReader("world")); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err = client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download after Update returned error: %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("Update mismatch: want %q got %q", "world", string(got))
	}
}

func TestClientUploadFileUpdateFile(t *testing.T) {
	client, prefix := newRealClient(t)
	ctx := context.Background()
	cleanup := registerCleanup(t, client)
	key := prefix + "/upload/file.txt"
	cleanup(key)

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")

	if err := os.WriteFile(firstPath, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile first failed: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile second failed: %v", err)
	}

	if err := client.UploadFile(ctx, key, firstPath); err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}

	got, err := client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("UploadFile mismatch: want %q got %q", "first", string(got))
	}

	if err := client.UpdateFile(ctx, key, secondPath); err != nil {
		t.Fatalf("UpdateFile returned error: %v", err)
	}

	got, err = client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download after UpdateFile returned error: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("UpdateFile mismatch: want %q got %q", "second", string(got))
	}
}
