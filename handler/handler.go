package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/pkg/meta"
	"io"
	"net/http"
	"os"
	"time"
)

// 上传文件
// 若为POST请求，则上传文件到本地，并将元信息存储到内存中(fileMetaMap Map变量)
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

func GetFileMetaHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	filehash := r.Form["filehash"]
	if len(filehash) == 0 {
		http.Error(w, "Missing filehash parameter", http.StatusBadRequest)
		return
	}
	fileSha1 := filehash[0]

	fmeta, ok := meta.GetFileMeta(fileSha1)
	if !ok {
		http.Error(w, "Get FileMeta Failed", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(fmeta)
	if err != nil {
		http.Error(w, "Json marshal error  ", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	filesha1 := r.Form.Get("filehash")

	fmeta, ok := meta.GetFileMeta(filesha1)
	if !ok {
		http.Error(w, "Get FileMeta Failed", http.StatusInternalServerError)
		return
	}

	// os.Open(fmeta.Location) 从Location打开存储在运行本进程的服务器磁盘上的文件
	// file表示打开的文件对应的句柄
	file, err := os.Open(fmeta.Location)
	if err != nil {
		http.Error(w, "Get FileMeta Failed", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Read File Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment;filename=\""+fmeta.FileName+"\"")
	w.Write(data)

}
