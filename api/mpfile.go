package api

import (
	"filestore-server/pkg/dao"
	"filestore-server/pkg/mw"
	redispool "filestore-server/pkg/redis"
	"filestore-server/service"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
)

func UploadInitMutipart(c *gin.Context) {
	infoVal, ok := c.Get(mw.CtxUploadInitKey)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing upload init params"})
		return
	}

	info, ok := infoVal.(*mw.MutiPartUploadInfo)
	if !ok || info == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload init params"})
		return
	}

	uploadID, err := service.NewUploadID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload id"})
		return
	}
	info.UploadID = uploadID

	conn := redispool.GetRedisConnectionPool().Get()
	defer conn.Close()

	conn.Do("HSET", "MP_"+info.UploadID, "chunkcount", info.ChunkCount)
	conn.Do("HSET", "MP_"+info.UploadID, "filehash", info.FileHash)
	conn.Do("HSET", "MP_"+info.UploadID, "filesize", info.FileSize)
	conn.Do("EXPIRE", "MP_"+info.UploadID, 86400)

	c.JSON(http.StatusOK, gin.H{"data": info})

}

func UploadPartHandler(c *gin.Context) {
	uploadID := c.GetString(mw.CtxUploadIDKey)
	chunkIndex := c.GetInt(mw.CtxChunkIndexKey)

	fpath := "/data/" + uploadID + "/" + strconv.Itoa(chunkIndex)
	fd, err := os.Create(fpath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Create file path error"})
	}
	defer fd.Close()
	if _, err := io.Copy(fd, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy data to path"})
	}

	conn := redispool.GetRedisConnectionPool().Get()
	defer conn.Close()

	conn.Do("HSET", "MP_"+uploadID, "chkidx_"+strconv.Itoa(chunkIndex), 1)

	c.JSON(http.StatusOK, gin.H{})

}

func CompleteUploadHandler(c *gin.Context) {
	infoVal, ok := c.Get(mw.CtxUploadCompleteKey)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing complete upload params"})
		return
	}

	info, ok := infoVal.(*mw.MutiPartUploadInfo)
	if !ok || info == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid complete upload params"})
		return
	}

	username := c.GetString(mw.CtxUsernameKey)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing username parameter"})
		return
	}
	filename := c.GetString(mw.CtxFilenameKey)
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename parameter"})
		return
	}

	conn := redispool.GetRedisConnectionPool().Get()
	defer conn.Close()

	infoObject, err := redis.StringMap(conn.Do("HGETALL", "MP_"+info.UploadID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get mutipartupload info "})
		return
	}
	if len(infoObject) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload session not found"})
		return
	}

	totalCount, err := strconv.Atoi(infoObject["chunkcount"])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid chunk count"})
		return
	}

	completedCount := 0
	for k, v := range infoObject {
		if strings.HasPrefix(k, "chkidx_") && v == "1" {
			completedCount++
		}
	}

	if completedCount != totalCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload chunks not complete"})
		return
	}

	location := "/data/" + info.UploadID
	if err := dao.SaveFileMeta(c.Request.Context(), info.FileHash, filename, info.FileSize, location); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file meta"})
		return
	}
	if err := service.SaveUserFileMeta(c.Request.Context(), username, info.FileHash, info.FileSize, filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save user file meta"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"upload":   info,
			"username": username,
			"filename": filename,
		},
	})
}
