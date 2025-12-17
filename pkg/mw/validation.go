package mw

import (
	"encoding/hex"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CtxFileHashKey = "filehash"
	CtxFilenameKey = "filename"
	CtxOpKey       = "op"
)

func paramFromQueryOrPost(c *gin.Context, key string) string {
	if v := c.PostForm(key); v != "" {
		return v
	}
	return c.Query(key)
}

// RequireFileHash 校验 filehash 必填且为 40 位 sha1 hex，并写入 gin context。
func RequireFileHash() gin.HandlerFunc {
	return func(c *gin.Context) {
		filehash := paramFromQueryOrPost(c, "filehash")
		if filehash == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing filehash parameter"})
			return
		}

		normalized := strings.ToLower(filehash)
		if len(normalized) != 40 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid filehash"})
			return
		}
		if _, err := hex.DecodeString(normalized); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid filehash"})
			return
		}

		c.Set(CtxFileHashKey, normalized)
		c.Next()
	}
}

// RequireFilename 校验 filename 必填，并写入 gin context。
func RequireFilename() gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := paramFromQueryOrPost(c, "filename")
		if filename == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing filename parameter"})
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
