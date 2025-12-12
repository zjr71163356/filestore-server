package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"filestore-server/pkg/dao"
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
			http.Error(w, "Internal Server Error: failed to read index.html", http.StatusInternalServerError)
			return
		}
		w.Write(file)
	// 2. 如果为POST请求，执行文件上传处理
	case "POST":
		//由于是通过表单上传文件，因此需要调用r.FormFile()方法

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Bad Request: failed to get file from form", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 确保 tmp 目录存在
		if err := os.MkdirAll("./tmp", 0755); err != nil {
			http.Error(w, "Internal Server Error: failed to create tmp dir", http.StatusInternalServerError)
			return
		}

		location := "./tmp/" + header.Filename
		dst, err := os.Create(location)
		if err != nil {
			http.Error(w, "Internal Server Error: failed to create file "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		hash := sha1.New()
		filesize, err := io.Copy(io.MultiWriter(dst, hash), file)
		if err != nil {
			http.Error(w, "Internal Server Error: failed to save file "+err.Error(), http.StatusInternalServerError)
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

		meta.InsertFileMeta(fmeta)

		data, err := json.Marshal(fmeta)
		if err != nil {
			http.Error(w, "Internal Server Error: failed to marshal file meta "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Upload File Success! " + string(data)))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)

	}
}

// 获取整个文件元信息fileMeta
// 通过filehash，即文件元信息的其中一个字段fsha1(fileMetaMap Map 中的key)
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
		http.Error(w, "Get FileMeta Failed: meta not found", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(fmeta)
	if err != nil {
		http.Error(w, "Internal Server Error: failed to marshal file meta", http.StatusInternalServerError)
		return
	}
	w.Write(data)

}

// 下载文件
// 通过filehash，即文件元信息的其中一个字段fsha1(fileMetaMap Map 中的key)
func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	filesha1 := r.Form.Get("filehash")

	fmeta, ok := meta.GetFileMeta(filesha1)
	if !ok {
		http.Error(w, "Get FileMeta Failed: meta not found", http.StatusInternalServerError)
		return
	}

	// os.Open(fmeta.Location) 从Location打开存储在运行本进程的服务器磁盘上的文件
	// file表示打开的文件对应的句柄
	file, err := os.Open(fmeta.Location)
	if err != nil {
		http.Error(w, "Get FileMeta Failed: failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Read File Failed: failed to read file content", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment;filename=\""+fmeta.FileName+"\"")
	w.Write(data)

}

// FileMetaUpdateHandler 更新元信息接口(重命名)
func FileMetaUpdateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	filesha1 := r.Form.Get("filehash")
	newFileName := r.Form.Get("filename")
	opType := r.Form.Get("op")
	if opType != "0" {
		http.Error(w, "Forbidden: invalid operation type", http.StatusForbidden)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Forbidden: Method Error", http.StatusForbidden)
		return
	}
	curFileMeta, ok := meta.GetFileMeta(filesha1)
	if !ok {
		http.Error(w, "Get FileMeta Failed: meta not found", http.StatusInternalServerError)
		return
	}
	curFileMeta.FileName = newFileName
	meta.UpdateFileMeta(curFileMeta)

	data, err := json.Marshal(curFileMeta)
	if err != nil {
		http.Error(w, "Internal Server Error: failed to marshal file meta", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

// FileDeleteHandler : 删除文件元信息
func FileDeleteHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	filesha1 := r.Form.Get("filehash")
	fmeta, ok := meta.GetFileMeta(filesha1)
	if !ok {
		http.Error(w, "Get FileMeta Failed: meta not found", http.StatusInternalServerError)
	}
	os.Remove(fmeta.Location)
	meta.RemoveFileMeta(filesha1)
	w.WriteHeader(http.StatusOK)
}
