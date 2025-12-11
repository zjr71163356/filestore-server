package handler_test

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
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

func TestGetFileMetaHandler(t *testing.T) {
	// Setup
	fileSha1 := "testsha1_get"
	expectedMeta := meta.FileMeta{
		FileSha1: fileSha1,
		FileName: "test_get.txt",
		FileSize: 123,
		Location: "/tmp/test_get.txt",
		UploadAt: "2023-01-01 10:00:00",
	}
	meta.UpdateFileMeta(expectedMeta)

	// Request
	// 注意：GetFileMetaHandler 使用 r.ParseForm() 解析参数，支持 URL query 参数
	req := httptest.NewRequest("POST", "/file/meta?filehash="+fileSha1, nil)
	rr := httptest.NewRecorder()

	// Execute
	handler.GetFileMetaHandler(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var gotMeta meta.FileMeta
	if err := json.Unmarshal(rr.Body.Bytes(), &gotMeta); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if gotMeta.FileSha1 != expectedMeta.FileSha1 {
		t.Errorf("FileSha1 mismatch: got %v want %v", gotMeta.FileSha1, expectedMeta.FileSha1)
	}
	if gotMeta.FileName != expectedMeta.FileName {
		t.Errorf("FileName mismatch: got %v want %v", gotMeta.FileName, expectedMeta.FileName)
	}
}

func TestDownloadFileHandler(t *testing.T) {
	// Setup
	tmpDir := "./tmp_download"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("download test content")
	filename := "download_test.txt"
	filePath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fileSha1 := "downloadsha1"
	fmeta := meta.FileMeta{
		FileSha1: fileSha1,
		FileName: filename,
		FileSize: int64(len(content)),
		Location: filePath,
	}
	meta.UpdateFileMeta(fmeta)

	// Request
	req := httptest.NewRequest("POST", "/file/download?filehash="+fileSha1, nil)
	rr := httptest.NewRecorder()

	// Execute
	handler.DownloadFileHandler(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if body := rr.Body.String(); body != string(content) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			body, string(content))
	}

	// Check Headers
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/octet-stream" {
		t.Errorf("Content-Type mismatch: got %v want %v", contentType, "application/octet-stream")
	}
	expectedDisposition := "attachment;filename=\"" + filename + "\""
	if disposition := rr.Header().Get("Content-Disposition"); disposition != expectedDisposition {
		t.Errorf("Content-Disposition mismatch: got %v want %v", disposition, expectedDisposition)
	}
}
