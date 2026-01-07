package s3util

import (
	"context"
	"testing"
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
