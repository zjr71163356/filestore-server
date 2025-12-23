package test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"filestore-server/pkg/dao"
	"filestore-server/pkg/db"
	"filestore-server/pkg/router"

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

	userFileTableDDL = `
CREATE TABLE IF NOT EXISTS tbl_user_file (
  id int(11) NOT NULL PRIMARY KEY AUTO_INCREMENT,
  user_name varchar(64) NOT NULL,
  file_sha1 varchar(64) NOT NULL DEFAULT '' COMMENT '文件hash',
  file_size bigint(20) DEFAULT '0' COMMENT '文件大小',
  file_name varchar(256) NOT NULL DEFAULT '' COMMENT '文件名',
  upload_at datetime DEFAULT CURRENT_TIMESTAMP COMMENT '上传时间',
  last_update datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后修改时间',
  status int(11) NOT NULL DEFAULT '0' COMMENT '文件状态(0正常1已删除2禁用)',
  UNIQUE KEY idx_user_file (user_name, file_sha1),
  KEY idx_status (status),
  KEY idx_user_id (user_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
)

func requireDB(t *testing.T) {
	t.Helper()
	if db.DBconn() == nil {
		t.Error("db not available")
	}
	ensureTestTables(t)
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
	if _, err := conn.ExecContext(ctx, userFileTableDDL); err != nil {
		t.Fatalf("failed to ensure tbl_user_file: %v", err)
	}
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

func signupAndLogin(t *testing.T, r *gin.Engine) (*http.Cookie, string) {
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
	return cookies[0], username
}

func signupAndLoginClient(t *testing.T, baseURL string, client *http.Client) (*http.Cookie, string) {
	t.Helper()
	username := "user_" + randHex(6)
	password := "pass_" + randHex(6)
	form := url.Values{
		"username": {username},
		"password": {password},
	}

	signupReq, _ := http.NewRequest("POST", baseURL+"/user/signup", strings.NewReader(form.Encode()))
	signupReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signupResp, err := client.Do(signupReq)
	if err != nil {
		t.Fatalf("signup request failed: %v", err)
	}
	signupResp.Body.Close()
	if signupResp.StatusCode != http.StatusOK && signupResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("signup failed status: %d", signupResp.StatusCode)
	}

	loginReq, _ := http.NewRequest("POST", baseURL+"/user/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login failed status: %d body:%s", loginResp.StatusCode, string(body))
	}

	cookies := loginResp.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("no session cookie returned")
	}
	return cookies[0], username
}

func assertFileMeta(t *testing.T, fileSha1, expectedName string, expectedSize int64) {
	t.Helper()
	meta, err := dao.GetFileMeta(context.Background(), fileSha1)
	if err != nil {
		t.Fatalf("meta not found for sha1 %s: %v", fileSha1, err)
	}
	if meta.FileName != expectedName {
		t.Fatalf("file name mismatch: got %s want %s", meta.FileName, expectedName)
	}
	if meta.FileSize != expectedSize {
		t.Fatalf("file size mismatch: got %d want %d", meta.FileSize, expectedSize)
	}
}

func assertUserFileMeta(t *testing.T, username, fileSha1, expectedName string, expectedSize int64) {
	t.Helper()
	conn := db.DBconn()
	if conn == nil {
		t.Fatalf("db connection is nil")
	}
	var gotName string
	var gotSize int64
	err := conn.QueryRowContext(context.Background(),
		"select file_name, file_size from tbl_user_file where user_name=? and file_sha1=? and status=0",
		username, fileSha1).
		Scan(&gotName, &gotSize)
	if err != nil {
		t.Fatalf("user file meta not found: %v", err)
	}
	if gotName != expectedName {
		t.Fatalf("user file name mismatch: got %s want %s", gotName, expectedName)
	}
	if gotSize != expectedSize {
		t.Fatalf("user file size mismatch: got %d want %d", gotSize, expectedSize)
	}
}
