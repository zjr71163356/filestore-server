package handler_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/handler"
	"filestore-server/pkg/dao"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// 启动测试服务器
func startTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/file/upload", handler.UploadFileHandler)
	mux.HandleFunc("/file/meta", handler.GetFileMetaHandler)
	mux.HandleFunc("/file/download", handler.DownloadFileHandler)
	mux.HandleFunc("/file/update", handler.FileMetaUpdateHandler)
	mux.HandleFunc("/file/delete", handler.FileDeleteHandler)

	return httptest.NewServer(mux)
}

func TestE2E_UploadDownload(t *testing.T) {
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
	filename := "e2e_test.txt"
	newFilename := "e2e_test_renamed.txt"

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
	req, _ = http.NewRequest("POST", baseURL+"/file/meta?filehash="+expectedSha1, nil)
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
	req, _ = http.NewRequest("POST", baseURL+"/file/download?filehash="+expectedSha1, nil)
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
	req, _ = http.NewRequest("POST", baseURL+"/file/meta?filehash="+expectedSha1, nil)
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
