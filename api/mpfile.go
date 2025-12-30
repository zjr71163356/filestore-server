package api

import (
	"errors"
	"filestore-server/pkg/dao"
	"filestore-server/pkg/mw"
	redispool "filestore-server/pkg/redis"
	"filestore-server/service"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
)

func getChunkFilePath(uploadID string, chunkIndex int) string {
	return "/data/" + uploadID + "/" + strconv.Itoa(chunkIndex)
}

func getFilePath(uploadID string) string {
	return "/data/" + uploadID
}

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

	fpath := getChunkFilePath(uploadID, chunkIndex)

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

	location := getFilePath(info.UploadID)

	destFile, err := os.Create(location)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}

	err = parallelMergeMutiPartFile(destFile, info.UploadID, info.ChunkCount, int64(info.ChunkSize))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to merge mutipart file"})
		return
	}
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

func parallelMergeMutiPartFile(destFile *os.File, uploadID string, totalCount int, chunkSize int64) error {

	limit := make(chan struct{}, 5)
	errChan := make(chan error, totalCount)

	var wg sync.WaitGroup

	for i := 1; i <= totalCount; i++ {
		wg.Add(1)
		limit <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-limit }()

			fpath := getChunkFilePath(uploadID, idx)

			f, err := os.Open(fpath)
			if err != nil {
				errChan <- err
				return
			}

			defer f.Close()
			offset := int64(idx-1) * chunkSize
			offWriter := io.NewOffsetWriter(destFile, offset)
			_, err = io.Copy(offWriter, f)
			if err != nil {
				errChan <- err
				return
			}

		}(i)

	}

	wg.Wait()
	close(errChan)

	var errList error
	for e := range errChan {
		errList = errors.Join(errList, e)
	}

	if errList != nil {
		log.Printf("合并过程中发生多处错误: %v", errList)
		return errList // 此时返回的是包含所有错误信息的对象
	}
	return nil
}
