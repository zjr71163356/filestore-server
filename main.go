package main

import (
	"filestore-server/handler"
	"net/http"
)

func main() {
	http.HandleFunc("/file/upload", handler.UploadFileHandler)
	http.ListenAndServe(":8080", nil)
}
