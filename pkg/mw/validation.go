package mw

import (
	"encoding/hex"
	"net/http"
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
