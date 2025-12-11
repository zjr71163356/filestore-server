package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"filestore-server/pkg/meta"
	"io"
	"net/http"
	"os"
	"time"
)

// 上传文件
func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// 1.如果为GET请求，响应本地的index.html上传页面
	switch r.Method {
	case "GET":
		file, err := os.ReadFile("./static/view/index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Write(file)
	// 2. 如果为POST请求，执行文件上传处理
	case "POST":
		//由于是通过表单上传文件，因此需要调用r.FormFile()方法

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		defer file.Close()
		location := "./tmp/" + header.Filename
		dst, err := os.Create(location)
		if err != nil {
			http.Error(w, "Internal Server Error:"+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		hash := sha1.New()
		filesize, err := io.Copy(io.MultiWriter(dst, hash), file)
		if err != nil {
			http.Error(w, "Internal Server Error:"+err.Error(), http.StatusInternalServerError)
			return
		}
		fileSha1 := hex.EncodeToString(hash.Sum(nil))

		fmeta := meta.FileMeta{
			FileSha1: fileSha1,
			FileName: header.Filename,
			FileSize: filesize,
			Location: location,
			UploadAt: time.Now().Format("2006-01-02 15:04:05"),
		}

		meta.UpdateFileMeta(fmeta)

		w.Write([]byte("Upload File Success!"))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)

	}
}
