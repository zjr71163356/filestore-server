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
	"os"
	"testing"
	"time"
)

// 启动测试服务器
func startTestServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/file/upload", handler.UploadFileHandler)
	mux.HandleFunc("/file/meta", handler.GetFileMetaHandler)
	mux.HandleFunc("/file/download", handler.DownloadFileHandler)

	server := &http.Server{
		Addr:    ":8081", // 使用不同于生产环境的端口
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// 简单等待服务器启动
	time.Sleep(100 * time.Millisecond)
	return server
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

	baseURL := "http://localhost:8081"
	client := &http.Client{}

	// 2. 准备测试数据
	content := []byte("E2E test content for full flow verification")
	h := sha1.New()
	io.WriteString(h, string(content))
	expectedSha1 := hex.EncodeToString(h.Sum(nil))
	filename := "e2e_test.txt"

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

	var metaData meta.FileMeta
	if err := json.NewDecoder(resp.Body).Decode(&metaData); err != nil {
		t.Fatalf("decode meta failed: %v", err)
	}

	if metaData.FileSha1 != expectedSha1 {
		t.Errorf("meta sha1 mismatch: got %s want %s", metaData.FileSha1, expectedSha1)
	}

	// 5. Step 3: 下载文件
	t.Log("Step 3: Downloading file...")
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

	t.Log("E2E Test Passed!")
}
