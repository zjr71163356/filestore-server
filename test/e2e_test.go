package test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/pkg/dao"
	"filestore-server/pkg/router"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 启动测试服务器
func startTestServer() *httptest.Server {
	gin.SetMode(gin.TestMode)
	r := router.New()
	return httptest.NewServer(r)
}

func signupAndLoginClient(t *testing.T, baseURL string, client *http.Client) *http.Cookie {
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
	return cookies[0]
}

func TestE2E_UploadDownload(t *testing.T) {
	requireDB(t)

	// 1. 准备环境
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 启动真实服务器
	server := startTestServer()
	defer server.Close()

	baseURL := server.URL
	client := server.Client()
	sessionCookie := signupAndLoginClient(t, baseURL, client)

	// 2. 准备测试数据
	content := make([]byte, 64)
	if _, err := rand.Read(content); err != nil {
		t.Fatalf("failed to generate random content: %v", err)
	}
	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("failed to hash content: %v", err)
	}
	expectedSha1 := hex.EncodeToString(h.Sum(nil))
	filename := "e2e_" + randHex(4) + ".txt"
	newFilename := "e2e_renamed_" + randHex(4) + ".txt"

	// 3. Step 1: 上传文件
	t.Log("Step 1: Uploading file...")
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	part.Write(content)
	w.Close()

	req, _ := http.NewRequest("POST", baseURL+"/file/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(sessionCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload failed status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 4. Step 2: 查询元信息
	t.Log("Step 2: Checking file meta...")
	req, _ = http.NewRequest("GET", baseURL+"/file/meta?filehash="+expectedSha1, nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("get meta request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get meta failed status: %d", resp.StatusCode)
	}

	var metaData dao.FileMeta
	if err := json.NewDecoder(resp.Body).Decode(&metaData); err != nil {
		t.Fatalf("decode meta failed: %v", err)
	}

	if metaData.FileSha1 != expectedSha1 {
		t.Errorf("meta sha1 mismatch: got %s want %s", metaData.FileSha1, expectedSha1)
	}
	if metaData.FileName != filename {
		t.Errorf("meta filename mismatch: got %s want %s", metaData.FileName, filename)
	}

	// 5. Step 3: 更新文件元信息（重命名）
	t.Log("Step 3: Renaming file meta...")
	req, _ = http.NewRequest("POST", baseURL+"/file/update?op=0&filehash="+expectedSha1+"&filename="+newFilename, nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update meta request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("update meta failed status: %d, body: %s", resp.StatusCode, string(body))
	}

	var updatedMeta dao.FileMeta
	if err := json.NewDecoder(resp.Body).Decode(&updatedMeta); err != nil {
		t.Fatalf("decode updated meta failed: %v", err)
	}
	if updatedMeta.FileName != newFilename {
		t.Errorf("updated meta filename mismatch: got %s want %s", updatedMeta.FileName, newFilename)
	}

	// 6. Step 4: 下载文件
	t.Log("Step 4: Downloading file...")
	req, _ = http.NewRequest("GET", baseURL+"/file/download?filehash="+expectedSha1, nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download failed status: %d", resp.StatusCode)
	}

	downloadedContent, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read download body failed: %v", err)
	}

	if string(downloadedContent) != string(content) {
		t.Errorf("downloaded content mismatch")
	}
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "attachment;filename=\""+newFilename+"\"" {
		t.Errorf("download disposition mismatch: got %s want %s", disposition, "attachment;filename=\""+newFilename+"\"")
	}

	// 7. Step 5: 删除文件
	t.Log("Step 5: Deleting file...")
	req, _ = http.NewRequest("POST", baseURL+"/file/delete?filehash="+expectedSha1, nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete failed status: %d, body: %s", resp.StatusCode, string(body))
	}
	if _, err := os.Stat("./tmp/" + filename); !os.IsNotExist(err) {
		t.Errorf("file was not removed from disk")
	}

	// 再次查询元信息，确认已删除
	req, _ = http.NewRequest("GET", baseURL+"/file/meta?filehash="+expectedSha1, nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("get meta after delete request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected meta query to fail after delete, got status %d", resp.StatusCode)
	}

	t.Log("E2E Test Passed!")
}
