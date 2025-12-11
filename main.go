package main

import (
	"filestore-server/handler"
	"net/http"
)

func main() {
	http.HandleFunc("/file/upload", handler.UploadFileHandler)
	http.HandleFunc("/file/meta", handler.GetFileMetaHandler)
	http.HandleFunc("/file/download", handler.DownloadFileHandler)
	http.HandleFunc("/file/update", handler.FileMetaUpdateHandler)
	http.HandleFunc("/file/delete", handler.FileDeleteHandler)
	http.ListenAndServe(":8080", nil)
}
