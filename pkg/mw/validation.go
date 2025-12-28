package mw

import (
	"encoding/hex"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CtxFileHashKey       = "filehash"
	CtxFilenameKey       = "filename"
	CtxOpKey             = "op"
	CtxUsernameKey       = "user_name"
	CtxFileSizeKey       = "file_size"
	CtxChunkCountKey     = "chunk_count"
	CtxChunkSizeKey      = "chunk_size"
	CtxUploadInitKey     = "upload_init"
	CtxUploadCompleteKey = "upload_complete"
	CtxUploadIDKey       = "uploadid"
	CtxChunkIndexKey     = "index"
)

type MutiPartUploadInfo struct {
	UploadID   string
	FileSize   int64
	ChunkCount int
	ChunkSize  int
	FileHash   string
}

// abortWithError 统一返回 400 JSON 并终止后续处理。
func abortWithError(c *gin.Context, msg string) bool {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
	return false
}

// paramFromQueryOrPost 从 gin.Context 中按优先级获取给定键的参数值。
// 优先从 POST 表单（c.PostForm(key)）读取；如果该值非空则返回之。
// 否则退回到 URL 查询参数（c.Query(key)）并返回其值。
// 如果两者都不存在或均为空，则返回空字符串。
func paramFromQueryOrPost(c *gin.Context, key string) string {
	if v := c.PostForm(key); v != "" {
		return v
	}
	return c.Query(key)
}

func mustFileHash(c *gin.Context) (string, bool) {
	filehash := strings.TrimSpace(paramFromQueryOrPost(c, "filehash"))
	if filehash == "" {
		return "", abortWithError(c, "missing filehash parameter")
	}

	normalized := strings.ToLower(filehash)
	if len(normalized) != 40 {
		return "", abortWithError(c, "invalid filehash")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", abortWithError(c, "invalid filehash")
	}

	return normalized, true
}

func mustFilename(c *gin.Context) (string, bool) {
	filename := strings.TrimSpace(paramFromQueryOrPost(c, "filename"))
	if filename == "" {
		return "", abortWithError(c, "missing filename parameter")
	}
	return filename, true
}

func mustUsername(c *gin.Context) (string, bool) {
	username := strings.TrimSpace(paramFromQueryOrPost(c, "username"))
	if username == "" {
		username = strings.TrimSpace(paramFromQueryOrPost(c, "user_name"))
	}
	if username == "" {
		return "", abortWithError(c, "missing username parameter")
	}
	return username, true
}

func mustFileSize(c *gin.Context) (int64, bool) {
	fileSizeStr := strings.TrimSpace(paramFromQueryOrPost(c, "filesize"))
	if fileSizeStr == "" {
		fileSizeStr = strings.TrimSpace(paramFromQueryOrPost(c, "file_size"))
	}
	if fileSizeStr == "" {
		return 0, abortWithError(c, "missing filesize parameter")
	}
	fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
	if err != nil || fileSize < 0 {
		return 0, abortWithError(c, "invalid filesize")
	}
	return fileSize, true
}

func mustChunkCount(c *gin.Context) (int, bool) {
	chunkCountStr := strings.TrimSpace(paramFromQueryOrPost(c, "chunkcount"))
	if chunkCountStr == "" {
		chunkCountStr = strings.TrimSpace(paramFromQueryOrPost(c, "chunk_count"))
	}
	if chunkCountStr == "" {
		return 0, abortWithError(c, "missing chunkcount parameter")
	}
	chunkCount, err := strconv.Atoi(chunkCountStr)
	if err != nil || chunkCount <= 0 {
		return 0, abortWithError(c, "invalid chunkcount")
	}
	return chunkCount, true
}

func mustChunkSize(c *gin.Context) (int, bool) {
	chunkSizeStr := strings.TrimSpace(paramFromQueryOrPost(c, "chunksize"))
	if chunkSizeStr == "" {
		chunkSizeStr = strings.TrimSpace(paramFromQueryOrPost(c, "chunk_size"))
	}
	if chunkSizeStr == "" {
		return 0, abortWithError(c, "missing chunksize parameter")
	}
	chunkSize, err := strconv.Atoi(chunkSizeStr)
	if err != nil || chunkSize <= 0 {
		return 0, abortWithError(c, "invalid chunksize")
	}
	return chunkSize, true
}

func mustUploadID(c *gin.Context) (string, bool) {
	uploadID := strings.TrimSpace(paramFromQueryOrPost(c, "uploadid"))
	if uploadID == "" {
		return "", abortWithError(c, "missing uploadid parameter")
	}
	return uploadID, true
}

func mustChunkIndex(c *gin.Context) (int, bool) {
	chunkIndexStr := strings.TrimSpace(paramFromQueryOrPost(c, "index"))
	if chunkIndexStr == "" {
		return 0, abortWithError(c, "missing index parameter")
	}
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		return 0, abortWithError(c, "invalid index")
	}
	return chunkIndex, true
}

// RequireFileHash 校验 filehash 必填且为 40 位 sha1 hex，并写入 gin context。
func RequireFileHash() gin.HandlerFunc {
	return func(c *gin.Context) {
		filehash, ok := mustFileHash(c)
		if !ok {
			return
		}
		c.Set(CtxFileHashKey, filehash)
		c.Next()
	}
}

// RequireFilename 校验 filename 必填，并写入 gin context。
func RequireFilename() gin.HandlerFunc {
	return func(c *gin.Context) {
		filename, ok := mustFilename(c)
		if !ok {
			return
		}
		c.Set(CtxFilenameKey, filename)
		c.Next()
	}
}

// RequireOp 校验 op 值与预期一致，并写入 gin context。
func RequireOp(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		op := paramFromQueryOrPost(c, "op")
		if op != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid operation type"})
			return
		}
		c.Set(CtxOpKey, op)
		c.Next()
	}
}

// RequireUploadFile 校验 multipart/form-data 中指定的文件字段存在，并将安全文件名写入 gin context。
func RequireUploadFile(fieldName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "content type must be multipart/form-data"})
			return
		}

		fileHeader, err := c.FormFile(fieldName)
		if err != nil || fileHeader == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to get file from form"})
			return
		}

		filename := strings.TrimSpace(fileHeader.Filename)
		if filename == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
			return
		}

		normalized := strings.ReplaceAll(filename, "\\", "/")
		safe := strings.TrimSpace(path.Base(normalized))
		if safe == "" || safe == "." || safe == "/" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
			return
		}

		c.Set(CtxFilenameKey, safe)
		c.Next()
	}
}

func RequireUsername() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, ok := mustUsername(c)
		if !ok {
			return
		}
		c.Set(CtxUsernameKey, username)
		c.Next()
	}
}

func RequireUploadInitMutipart() gin.HandlerFunc {
	return func(c *gin.Context) {
		filehash, ok := mustFileHash(c)
		if !ok {
			return
		}
		fileSize, ok := mustFileSize(c)
		if !ok {
			return
		}
		chunkCount, ok := mustChunkCount(c)
		if !ok {
			return
		}
		chunkSize, ok := mustChunkSize(c)
		if !ok {
			return
		}

		info := &MutiPartUploadInfo{
			FileHash:   filehash,
			FileSize:   fileSize,
			ChunkCount: chunkCount,
			ChunkSize:  chunkSize,
		}
		c.Set(CtxUploadInitKey, info)
		c.Next()
	}
}

func RequireUploadPart() gin.HandlerFunc {
	return func(c *gin.Context) {
		uploadID, ok := mustUploadID(c)
		if !ok {
			return
		}
		chunkIndex, ok := mustChunkIndex(c)
		if !ok {
			return
		}

		c.Set(CtxUploadIDKey, uploadID)
		c.Set(CtxChunkIndexKey, chunkIndex)
		c.Next()
	}
}

func RequireUploadComplete() gin.HandlerFunc {
	return func(c *gin.Context) {
		uploadID, ok := mustUploadID(c)
		if !ok {
			return
		}
		username, ok := mustUsername(c)
		if !ok {
			return
		}
		filehash, ok := mustFileHash(c)
		if !ok {
			return
		}
		fileSize, ok := mustFileSize(c)
		if !ok {
			return
		}
		filename, ok := mustFilename(c)
		if !ok {
			return
		}
		chunkCount, ok := mustChunkCount(c)
		if !ok {
			return
		}
		chunkSize, ok := mustChunkSize(c)
		if !ok {
			return
		}

		info := &MutiPartUploadInfo{
			UploadID:   uploadID,
			FileHash:   filehash,
			FileSize:   fileSize,
			ChunkCount: chunkCount,
			ChunkSize:  chunkSize,
		}
		c.Set(CtxUsernameKey, username)
		c.Set(CtxFilenameKey, filename)
		c.Set(CtxUploadCompleteKey, info)
		c.Next()
	}
}
