package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewUploadID 生成分片上传会话的唯一 ID。
func NewUploadID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
