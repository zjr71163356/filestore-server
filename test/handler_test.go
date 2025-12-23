package test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/pkg/dao"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
	requireDB(t)

	r := newTestRouter()
	sessionCookie, username := signupAndLogin(t, r)

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
	req.AddCookie(sessionCookie)

	// 执行 handler
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	res := rr.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d body:%s", res.StatusCode, rr.Body.String())
	}

	assertFileMeta(t, expectedSha1, filename, int64(len(content)))

	// 检查物理文件存在
	if _, err := os.Stat(filepath.Join(tmpDir, filename)); err != nil {
		t.Errorf("uploaded file not found: %v", err)
	}

	assertUserFileMeta(t, username, expectedSha1, filename, int64(len(content)))
}

func TestGetFileMetaHandler(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

	// Setup
	fileSha1 := randHex(20)
	expectedMeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: "meta_" + randHex(4) + ".txt",
		FileSize: int64(len(fileSha1)),
		Location: "/tmp/" + randHex(6),
		UploadAt: "2023-01-01 10:00:00",
	}
	if err := dao.SaveFileMeta(context.Background(), expectedMeta.FileSha1, expectedMeta.FileName, expectedMeta.FileSize, expectedMeta.Location); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	req := httptest.NewRequest("GET", "/file/meta?filehash="+fileSha1, nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(rr, req)

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

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

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
	if err := dao.SaveFileMeta(context.Background(), fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	req := httptest.NewRequest("GET", "/file/download?filehash="+fileSha1, nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(rr, req)

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

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

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
	if err := dao.SaveFileMeta(context.Background(), fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	// op=0 表示重命名操作
	url := "/file/update?op=0&filehash=" + fileSha1 + "&filename=" + newName
	req := httptest.NewRequest("POST", url, nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(rr, req)

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
	assertFileMeta(t, fileSha1, newName, fmeta.FileSize)
}

func TestFileDeleteHandler(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

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
	if err := dao.SaveFileMeta(context.Background(), fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location); err != nil {
		t.Fatalf("failed to seed meta: %v", err)
	}

	// Request
	req := httptest.NewRequest("POST", "/file/delete?filehash="+fileSha1, nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(rr, req)

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
	if _, err := dao.GetFileMeta(context.Background(), fileSha1); err == nil {
		t.Errorf("meta was not deleted from memory")
	}
}
