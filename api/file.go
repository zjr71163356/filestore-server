package api

import (
	"filestore-server/pkg/dao"
	"filestore-server/pkg/mw"
	"filestore-server/service"
	"net/http"
	"os"

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

		fmeta, err := service.UploadFile(c.Request.Context(), file, header.Filename)
		if err != nil {
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
	fileSha1 := c.GetString(mw.CtxFileHashKey)

	fmeta, err := dao.GetFileMeta(c.Request.Context(), fileSha1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
		return
	}

	c.JSON(http.StatusOK, fmeta)
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	filesha1 := c.GetString(mw.CtxFileHashKey)

	fmeta, data, err := service.DownloadFile(c.Request.Context(), filesha1)
	if err != nil {
		if err.Error() == "file not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=\""+fmeta.FileName+"\"")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// FileMetaUpdate 更新元信息接口(重命名)
func FileMetaUpdate(c *gin.Context) {
	filesha1 := c.GetString(mw.CtxFileHashKey)
	newFileName := c.GetString(mw.CtxFilenameKey)

	curFileMeta, err := service.RenameFile(c.Request.Context(), filesha1, newFileName)
	if err != nil {
		if err.Error() == "file not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update file meta"})
		return
	}

	c.JSON(http.StatusOK, curFileMeta)
}

// FileDelete 删除文件元信息
func FileDelete(c *gin.Context) {
	filesha1 := c.GetString(mw.CtxFileHashKey)

	if err := service.DeleteFile(c.Request.Context(), filesha1); err != nil {
		if err.Error() == "file not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "meta not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "delete success"})
}
