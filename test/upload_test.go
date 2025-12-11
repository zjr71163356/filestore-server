package handler_test

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"filestore-server/handler"
	"filestore-server/pkg/meta"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// createUploadRequest 构造 multipart/form-data 的上传请求
func createUploadRequest(fieldName, filename string, content []byte) (*http.Request, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	w.Close()
	req := httptest.NewRequest("POST", "/file/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

func TestUploadFileHandler_UpdateMeta(t *testing.T) {
	// 准备临时目录
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("test content for hashing")
	h := sha1.New()
	io.WriteString(h, string(content))
	expectedSha1 := hex.EncodeToString(h.Sum(nil))
	filename := "testfile.txt"

	req, err := createUploadRequest("file", filename, content)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	// 执行 handler
	rr := httptest.NewRecorder()
	handler.UploadFileHandler(rr, req)

	res := rr.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d body:%s", res.StatusCode, rr.Body.String())
	}

	// 检查 meta 中是否存在对应条目（GetFileMeta 返回零值时表示未找到）
	gotMeta, ok := meta.GetFileMeta(expectedSha1)
	if !ok {
		t.Fatalf("meta not found for sha1 %s", expectedSha1)
	}
	if gotMeta.FileSha1 == "" {
		t.Fatalf("meta not found for sha1 %s", expectedSha1)
	}
	if gotMeta.FileName != filename {
		t.Errorf("FileName mismatch: got %q want %q", gotMeta.FileName, filename)
	}
	if gotMeta.FileSha1 != expectedSha1 {
		t.Errorf("FileSha1 mismatch: got %q want %q", gotMeta.FileSha1, expectedSha1)
	}

	// 检查物理文件存在
	if _, err := os.Stat(filepath.Join(tmpDir, filename)); err != nil {
		t.Errorf("uploaded file not found: %v", err)
	}
}
