package handler_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/handler"
	"filestore-server/pkg/dao"
	"filestore-server/pkg/db"
	"filestore-server/service"
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

func requireDB(t *testing.T) {
	t.Helper()
	if db.DBconn() == nil {
		t.Error("db not available")
	}
	
}

func randHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func TestUploadFileHandler_UpdateMeta(t *testing.T) {
	requireDB(t)

	// 准备临时目录
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte(randHex(16))
	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("failed to write content hash: %v", err)
	}
	expectedSha1 := hex.EncodeToString(h.Sum(nil))
	filename := "upload_" + randHex(4) + ".txt"

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
	gotMeta, err := service.GetFileMeta(context.Background(), expectedSha1)
	if err != nil {
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
	requireDB(t)

	// Setup
	fileSha1 := randHex(20)
	expectedMeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: "meta_" + randHex(4) + ".txt",
		FileSize: int64(len(fileSha1)),
		Location: "/tmp/" + randHex(6),
		UploadAt: "2023-01-01 10:00:00",
	}
	if err := service.SaveFileMeta(context.Background(), expectedMeta); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

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

	var gotMeta dao.FileMeta
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
	requireDB(t)

	// Setup
	tmpDir := "./tmp_download"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte(randHex(24))
	filename := "download_" + randHex(4) + ".txt"
	filePath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fileSha1 := randHex(20)
	fmeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: filename,
		FileSize: int64(len(content)),
		Location: filePath,
	}
	if err := service.SaveFileMeta(context.Background(), fmeta); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

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

func TestFileMetaUpdateHandler(t *testing.T) {
	requireDB(t)

	// Setup
	fileSha1 := randHex(20)
	originalName := "orig_" + randHex(4) + ".txt"
	newName := "renamed_" + randHex(4) + ".txt"

	fmeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: originalName,
		FileSize: 100,
		Location: "/tmp/" + randHex(6),
	}
	if err := service.SaveFileMeta(context.Background(), fmeta); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	// op=0 表示重命名操作
	url := "/file/update?op=0&filehash=" + fileSha1 + "&filename=" + newName
	req := httptest.NewRequest("POST", url, nil)
	rr := httptest.NewRecorder()

	// Execute
	handler.FileMetaUpdateHandler(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var gotMeta dao.FileMeta
	if err := json.Unmarshal(rr.Body.Bytes(), &gotMeta); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if gotMeta.FileName != newName {
		t.Errorf("FileName mismatch: got %v want %v", gotMeta.FileName, newName)
	}

	// Verify internal state
	storedMeta, err := service.GetFileMeta(context.Background(), fileSha1)
	if err != nil {
		t.Fatalf("meta not found after update")
	}
	if storedMeta.FileName != newName {
		t.Errorf("Stored FileName mismatch: got %v want %v", storedMeta.FileName, newName)
	}
}

func TestFileDeleteHandler(t *testing.T) {
	requireDB(t)

	// Setup
	tmpDir := "./tmp_delete"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("content_" + randHex(6))
	filename := "delete_" + randHex(4) + ".txt"
	filePath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	fileSha1 := randHex(20)
	fmeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: filename,
		FileSize: int64(len(content)),
		Location: filePath,
	}
	if err := service.SaveFileMeta(context.Background(), fmeta); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	req := httptest.NewRequest("POST", "/file/delete?filehash="+fileSha1, nil)
	rr := httptest.NewRecorder()

	// Execute
	handler.FileDeleteHandler(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify file is deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("file was not deleted from disk")
	}

	// Verify meta is deleted
	if _, err := service.GetFileMeta(context.Background(), fileSha1); err == nil {
		t.Errorf("meta was not deleted from memory")
	}
}
