package handler

import (
	"io"
	"net/http"
	"os"
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
		dst, err := os.Create("./tmp/" + header.Filename)
		if err != nil {
			http.Error(w, "Internal Server Error:"+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		_, err = io.Copy(dst, file)
		if err != nil {
			http.Error(w, "Internal Server Error:"+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Upload File Success!"))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)

	}
}
