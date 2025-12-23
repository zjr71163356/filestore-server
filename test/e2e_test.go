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
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// 启动测试服务器
func startTestServer() *httptest.Server {
	gin.SetMode(gin.TestMode)
	r := router.New()
	return httptest.NewServer(r)
}

func uploadFile(t *testing.T, client *http.Client, baseURL string, sessionCookie *http.Cookie, filename string, content []byte) (string, int64) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write content failed: %v", err)
	}
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

	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("hash content failed: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil)), int64(len(content))
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
	sessionCookie, username := signupAndLoginClient(t, baseURL, client)

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

	assertUserFileMeta(t, username, expectedSha1, filename, int64(len(content)))

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

func TestE2E_UserFilelistPagination(t *testing.T) {
	requireDB(t)

	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	server := startTestServer()
	defer server.Close()

	baseURL := server.URL
	client := server.Client()
	sessionCookie, username := signupAndLoginClient(t, baseURL, client)

	seed := []struct {
		FileName   string
		Content    []byte
		LastUpdate string
	}{
		{
			FileName:   "e2e_list_old_" + randHex(4) + ".txt",
			Content:    []byte("old_" + randHex(8)),
			LastUpdate: "2023-01-01 10:00:00",
		},
		{
			FileName:   "e2e_list_mid_" + randHex(4) + ".txt",
			Content:    []byte("mid_" + randHex(8)),
			LastUpdate: "2023-01-02 10:00:00",
		},
		{
			FileName:   "e2e_list_new_" + randHex(4) + ".txt",
			Content:    []byte("new_" + randHex(8)),
			LastUpdate: "2023-01-03 10:00:00",
		},
	}

	type uploaded struct {
		FileSha1   string
		LastUpdate string
	}
	var uploadedFiles []uploaded

	for _, item := range seed {
		fileSha1, _ := uploadFile(t, client, baseURL, sessionCookie, item.FileName, item.Content)
		uploadedFiles = append(uploadedFiles, uploaded{
			FileSha1:   fileSha1,
			LastUpdate: item.LastUpdate,
		})
	}

	conn := db.DBconn()
	if conn == nil {
		t.Fatalf("db connection is nil")
	}

	ctx := context.Background()
	for _, item := range uploadedFiles {
		_, err := conn.ExecContext(ctx,
			"update tbl_user_file set last_update=? where user_name=? and file_sha1=?",
			item.LastUpdate, username, item.FileSha1)
		if err != nil {
			t.Fatalf("failed to update last_update: %v", err)
		}
	}

	req, _ := http.NewRequest("POST", baseURL+"/user/filelist?user_name="+username+"&limit=2&offset=1", nil)
	req.AddCookie(sessionCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("filelist request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("filelist failed status: %d, body: %s", resp.StatusCode, string(body))
	}

	var body struct {
		Total int            `json:"total"`
		Files []dao.FileMeta `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if body.Total != len(seed) {
		t.Fatalf("total mismatch: got %d want %d", body.Total, len(seed))
	}
	if len(body.Files) != 2 {
		t.Fatalf("files length mismatch: got %d want %d", len(body.Files), 2)
	}

	if body.Files[0].FileSha1 != uploadedFiles[1].FileSha1 {
		t.Errorf("first file sha mismatch: got %s want %s", body.Files[0].FileSha1, uploadedFiles[1].FileSha1)
	}
	if body.Files[1].FileSha1 != uploadedFiles[0].FileSha1 {
		t.Errorf("second file sha mismatch: got %s want %s", body.Files[1].FileSha1, uploadedFiles[0].FileSha1)
	}
	if body.Files[0].UploadAt != uploadedFiles[1].LastUpdate {
		t.Errorf("first file last_update mismatch: got %s want %s", body.Files[0].UploadAt, uploadedFiles[1].LastUpdate)
	}
	if body.Files[1].UploadAt != uploadedFiles[0].LastUpdate {
		t.Errorf("second file last_update mismatch: got %s want %s", body.Files[1].UploadAt, uploadedFiles[0].LastUpdate)
	}

	if _, err := time.Parse("2006-01-02 15:04:05", body.Files[0].UploadAt); err != nil {
		t.Errorf("invalid upload_at format: %v", err)
	}
}
