package test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/pkg/dao"
	"filestore-server/pkg/db"
	"filestore-server/pkg/router"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	userTableDDL = `
CREATE TABLE IF NOT EXISTS tbl_user (
  id int(11) NOT NULL AUTO_INCREMENT,
  user_name varchar(64) NOT NULL DEFAULT '' COMMENT '用户名',
  user_pwd varchar(256) NOT NULL DEFAULT '' COMMENT '用户encoded密码',
  email varchar(64) DEFAULT '' COMMENT '邮箱',
  phone varchar(128) DEFAULT '' COMMENT '手机号',
  email_validated tinyint(1) DEFAULT 0 COMMENT '邮箱是否已验证',
  phone_validated tinyint(1) DEFAULT 0 COMMENT '手机号是否已验证',
  signup_at datetime DEFAULT CURRENT_TIMESTAMP COMMENT '注册日期',
  last_active datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后活跃时间戳',
  profile text COMMENT '用户属性',
  status int(11) NOT NULL DEFAULT '0' COMMENT '账户状态(启用/禁用/锁定/标记删除等)',
  PRIMARY KEY (id),
  UNIQUE KEY idx_username (user_name),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	fileTableDDL = `
CREATE TABLE IF NOT EXISTS tbl_file (
  id int(11) NOT NULL AUTO_INCREMENT,
  file_sha1 char(40) NOT NULL,
  file_name varchar(256) NOT NULL,
  file_size bigint DEFAULT 0,
  file_addr varchar(1024) NOT NULL,
  status int NOT NULL DEFAULT 1,
  upload_at datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY idx_file_sha1 (file_sha1),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
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
	ensureTestTables(t)
}

func randHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return router.New()
}

// ensureTestTables creates tables for tests if they do not exist.
func ensureTestTables(t *testing.T) {
	t.Helper()
	conn := db.DBconn()
	if conn == nil {
		t.Fatalf("db connection is nil")
	}
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, userTableDDL); err != nil {
		t.Fatalf("failed to ensure tbl_user: %v", err)
	}
	if _, err := conn.ExecContext(ctx, fileTableDDL); err != nil {
		t.Fatalf("failed to ensure tbl_file: %v", err)
	}
}

func signupAndLogin(t *testing.T, r *gin.Engine) *http.Cookie {
	t.Helper()
	username := "user_" + randHex(6)
	password := "pass_" + randHex(6)

	form := url.Values{
		"username": {username},
		"password": {password},
	}

	signupReq := httptest.NewRequest("POST", "/user/signup", strings.NewReader(form.Encode()))
	signupReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signupRes := httptest.NewRecorder()
	r.ServeHTTP(signupRes, signupReq)
	if signupRes.Code != http.StatusOK && signupRes.Code != http.StatusBadRequest {
		t.Fatalf("signup failed: status %d body %s", signupRes.Code, signupRes.Body.String())
	}

	loginReq := httptest.NewRequest("POST", "/user/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login failed: status %d body %s", loginRes.Code, loginRes.Body.String())
	}
	resp := loginRes.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("no session cookie returned")
	}
	return cookies[0]
}

func TestUploadFileHandler_UpdateMeta(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie := signupAndLogin(t, r)

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

	// 检查 meta 中是否存在对应条目（GetFileMeta 返回零值时表示未找到）
	gotMeta, err := dao.GetFileMeta(context.Background(), expectedSha1)
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

	r := newTestRouter()
	sessionCookie := signupAndLogin(t, r)

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
	sessionCookie := signupAndLogin(t, r)

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
	sessionCookie := signupAndLogin(t, r)

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
	storedMeta, err := dao.GetFileMeta(context.Background(), fileSha1)
	if err != nil {
		t.Fatalf("meta not found after update")
	}
	if storedMeta.FileName != newName {
		t.Errorf("Stored FileName mismatch: got %v want %v", storedMeta.FileName, newName)
	}
}

func TestFileDeleteHandler(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie := signupAndLogin(t, r)

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
