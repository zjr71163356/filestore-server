package api

import (
	"crypto/sha1"
	"encoding/hex"
	"filestore-server/pkg/dao"
	"filestore-server/service"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// 上传文件
// GET 返回上传页；POST 上传文件并存储元信息。
func UploadFile(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		file, err := os.ReadFile("./static/view/index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read index.html"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", file)
	case http.MethodPost:
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get file from form"})
			return
		}
		defer file.Close()

		if err := os.MkdirAll("./tmp", 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tmp dir"})
			return
		}

		location := "./tmp/" + header.Filename
		dst, err := os.Create(location)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create file"})
			return
		}
		defer dst.Close()

		hash := sha1.New()
		filesize, err := io.Copy(io.MultiWriter(dst, hash), file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}
		fileSha1 := hex.EncodeToString(hash.Sum(nil))

		fmeta := dao.FileMeta{
			FileSha1: fileSha1,
			FileName: header.Filename,
			FileSize: filesize,
			Location: location,
			UploadAt: time.Now().Format("2006-01-02 15:04:05"),
		}

		if err := service.SaveFileMeta(c.Request.Context(), fmeta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist file meta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "upload file success", "file": fmeta})
	default:
		c.Status(http.StatusMethodNotAllowed)
	}
}

// 获取文件元信息
func GetFileMeta(c *gin.Context) {
	fileSha1 := c.Query("filehash")
	if fileSha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filehash parameter"})
		return
	}

	fmeta, err := service.GetFileMeta(c.Request.Context(), fileSha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
		return
	}

	c.JSON(http.StatusOK, fmeta)
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	filesha1 := c.Query("filehash")
	if filesha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filehash parameter"})
		return
	}

	fmeta, err := service.GetFileMeta(c.Request.Context(), filesha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
		return
	}

	file, err := os.Open(fmeta.Location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=\""+fmeta.FileName+"\"")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// FileMetaUpdate 更新元信息接口(重命名)
func FileMetaUpdate(c *gin.Context) {
	filesha1 := c.PostForm("filehash")
	if filesha1 == "" {
		filesha1 = c.Query("filehash")
	}
	newFileName := c.PostForm("filename")
	if newFileName == "" {
		newFileName = c.Query("filename")
	}
	opType := c.PostForm("op")
	if opType == "" {
		opType = c.Query("op")
	}
	if opType != "0" {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid operation type"})
		return
	}

	curFileMeta, err := service.GetFileMeta(c.Request.Context(), filesha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
		return
	}
	curFileMeta.FileName = newFileName

	if err := service.UpdateFileMeta(c.Request.Context(), curFileMeta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update file meta"})
		return
	}

	c.JSON(http.StatusOK, curFileMeta)
}

// FileDelete 删除文件元信息
func FileDelete(c *gin.Context) {
	filesha1 := c.PostForm("filehash")
	if filesha1 == "" {
		filesha1 = c.Query("filehash")
	}
	if filesha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filehash parameter"})
		return
	}

	fmeta, err := service.GetFileMeta(c.Request.Context(), filesha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
		return
	}

	if err := service.DeleteFileMeta(c.Request.Context(), filesha1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete meta"})
		return
	}

	if err := os.Remove(fmeta.Location); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "delete success"})
}
