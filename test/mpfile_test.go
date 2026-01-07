package test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"filestore-server/config"
	"filestore-server/pkg/dao"
	"filestore-server/pkg/mw"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	redigo "github.com/gomodule/redigo/redis"
)

func multipartBaseDir(t *testing.T) string {
	t.Helper()
	cfg := config.MustLoad()
	tmpDir := cfg.Storage.TmpDir
	if tmpDir == "" {
		tmpDir = "./tmp"
	}
	return tmpDir
}

func initMultipartUpload(t *testing.T, r *gin.Engine, sessionCookie *http.Cookie, filehash string, filesize int64, chunkCount, chunkSize int) mw.MutiPartUploadInfo {
	t.Helper()
	form := url.Values{
		"filehash":   {filehash},
		"filesize":   {strconv.FormatInt(filesize, 10)},
		"chunkcount": {strconv.Itoa(chunkCount)},
		"chunksize":  {strconv.Itoa(chunkSize)},
	}

	req := httptest.NewRequest("POST", "/mpfile/init", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("init multipart upload failed: status=%d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data mw.MutiPartUploadInfo `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode init response: %v", err)
	}
	if resp.Data.UploadID == "" {
		t.Fatalf("upload id should not be empty")
	}
	return resp.Data
}

func TestMultipartInit_ShouldPersistUploadSession(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

	content := []byte(randHex(12))
	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("hash content failed: %v", err)
	}
	filehash := hex.EncodeToString(h.Sum(nil))
	filesize := int64(len(content))
	chunkSize := 4
	chunkCount := (len(content) + chunkSize - 1) / chunkSize

	info := initMultipartUpload(t, r, sessionCookie, filehash, filesize, chunkCount, chunkSize)

	conn := requireRedis(t)
	uploadKey := "MP_" + info.UploadID
	t.Cleanup(func() {
		_, _ = conn.Do("DEL", uploadKey)
		conn.Close()
	})

	storedHash, err := redigo.String(conn.Do("HGET", uploadKey, "filehash"))
	if err != nil {
		t.Fatalf("failed to read filehash from redis: %v", err)
	}
	if storedHash != filehash {
		t.Errorf("filehash mismatch, got %s want %s", storedHash, filehash)
	}

	storedCount, err := redigo.Int(conn.Do("HGET", uploadKey, "chunkcount"))
	if err != nil {
		t.Fatalf("failed to read chunkcount from redis: %v", err)
	}
	if storedCount != chunkCount {
		t.Errorf("chunkcount mismatch, got %d want %d", storedCount, chunkCount)
	}

	storedSize, err := redigo.Int64(conn.Do("HGET", uploadKey, "filesize"))
	if err != nil {
		t.Fatalf("failed to read filesize from redis: %v", err)
	}
	if storedSize != filesize {
		t.Errorf("filesize mismatch, got %d want %d", storedSize, filesize)
	}
}

func TestMultipartUploadPart_WritesChunkAndMarksRedis(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, _ := signupAndLogin(t, r)

	uploadID := "ut_" + randHex(6)
	chunkDir := filepath.Join(multipartBaseDir(t), uploadID)
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("no permission to create chunk dir %s: %v", chunkDir, err)
		}
		t.Fatalf("failed to create chunk dir: %v", err)
	}
	defer os.RemoveAll(chunkDir)

	chunkData := []byte("chunk-" + randHex(8))

	req := httptest.NewRequest("POST", "/mpfile/upload?uploadid="+uploadID+"&index=1", bytes.NewReader(chunkData))
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload part failed: status=%d body=%s", rr.Code, rr.Body.String())
	}

	chunkPath := filepath.Join(chunkDir, "1")
	gotData, err := os.ReadFile(chunkPath)
	if err != nil {
		t.Fatalf("failed to read chunk file: %v", err)
	}
	if !bytes.Equal(gotData, chunkData) {
		t.Errorf("chunk data mismatch, got %q want %q", string(gotData), string(chunkData))
	}

	conn := requireRedis(t)
	uploadKey := "MP_" + uploadID
	t.Cleanup(func() {
		_, _ = conn.Do("DEL", uploadKey)
		conn.Close()
	})

	chunkMark, err := redigo.String(conn.Do("HGET", uploadKey, "chkidx_1"))
	if err != nil {
		t.Fatalf("failed to read chunk mark: %v", err)
	}
	if chunkMark != "1" {
		t.Errorf("chunk mark mismatch, got %s want %s", chunkMark, "1")
	}
}

func TestMultipartComplete_ReturnsBadRequestWhenIncomplete(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, username := signupAndLogin(t, r)

	content := []byte(randHex(24))
	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("hash content failed: %v", err)
	}
	filehash := hex.EncodeToString(h.Sum(nil))
	filesize := int64(len(content))
	chunkSize := 6
	chunkCount := (len(content) + chunkSize - 1) / chunkSize

	info := initMultipartUpload(t, r, sessionCookie, filehash, filesize, chunkCount, chunkSize)

	form := url.Values{
		"uploadid":   {info.UploadID},
		"user_name":  {username},
		"filehash":   {filehash},
		"filesize":   {strconv.FormatInt(filesize, 10)},
		"filename":   {"mp_" + randHex(4) + ".txt"},
		"chunkcount": {strconv.Itoa(chunkCount)},
		"chunksize":  {strconv.Itoa(chunkSize)},
	}

	req := httptest.NewRequest("POST", "/mpfile/complete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("complete should fail when chunks missing: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "upload chunks not complete") {
		t.Errorf("unexpected complete response: %s", rr.Body.String())
	}

	_, exists, err := dao.GetFileExist(context.Background(), filehash)
	if err != nil {
		t.Fatalf("query file exist failed: %v", err)
	}
	if exists {
		t.Fatalf("file meta should not be saved when chunks missing")
	}

	conn := requireRedis(t)
	uploadKey := "MP_" + info.UploadID
	t.Cleanup(func() {
		_, _ = conn.Do("DEL", uploadKey)
		conn.Close()
	})
}

func TestMultipartComplete_MergesChunksAndSavesMeta(t *testing.T) {
	requireDB(t)

	r := newTestRouter()
	sessionCookie, username := signupAndLogin(t, r)

	content := []byte("mp_full_" + randHex(24))
	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		t.Fatalf("hash content failed: %v", err)
	}
	filehash := hex.EncodeToString(h.Sum(nil))
	filesize := int64(len(content))
	chunkSize := 8
	chunkCount := (len(content) + chunkSize - 1) / chunkSize
	filename := "mp_full_" + randHex(4) + ".txt"

	info := initMultipartUpload(t, r, sessionCookie, filehash, filesize, chunkCount, chunkSize)
	chunkCount = info.ChunkCount
	chunkSize = info.ChunkSize

	uploadRoot := filepath.Join(multipartBaseDir(t), info.UploadID)
	if err := os.MkdirAll(uploadRoot, 0o755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("no permission to create upload dir %s: %v", uploadRoot, err)
		}
		t.Fatalf("failed to create upload dir: %v", err)
	}
	defer os.RemoveAll(uploadRoot)

	for idx := 0; idx < chunkCount; idx++ {
		start := idx * chunkSize
		end := start + chunkSize
		if end > len(content) {
			end = len(content)
		}

		req := httptest.NewRequest("POST", "/mpfile/upload?uploadid="+info.UploadID+"&index="+strconv.Itoa(idx+1), bytes.NewReader(content[start:end]))
		req.AddCookie(sessionCookie)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("upload chunk %d failed: status=%d body=%s", idx+1, rr.Code, rr.Body.String())
		}
	}

	form := url.Values{
		"uploadid":   {info.UploadID},
		"user_name":  {username},
		"filehash":   {filehash},
		"filesize":   {strconv.FormatInt(filesize, 10)},
		"filename":   {filename},
		"chunkcount": {strconv.Itoa(chunkCount)},
		"chunksize":  {strconv.Itoa(chunkSize)},
	}
	req := httptest.NewRequest("POST", "/mpfile/complete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("complete upload failed: status=%d body=%s", rr.Code, rr.Body.String())
	}

	mergedPath := filepath.Join(uploadRoot, filename)
	mergedData, err := os.ReadFile(mergedPath)
	if err != nil {
		t.Fatalf("failed to read merged file: %v", err)
	}
	if !bytes.Equal(mergedData, content) {
		t.Errorf("merged content mismatch, got %q want %q", string(mergedData), string(content))
	}

	assertFileMeta(t, filehash, filename, filesize)
	assertUserFileMeta(t, username, filehash, filename, filesize)

	conn := requireRedis(t)
	uploadKey := "MP_" + info.UploadID
	t.Cleanup(func() {
		_, _ = conn.Do("DEL", uploadKey)
		conn.Close()
	})
}
