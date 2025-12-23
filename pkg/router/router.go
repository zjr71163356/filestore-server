package router

import (
	"filestore-server/api"
	"filestore-server/pkg/mw"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// New 构建 gin.Engine，注册路由与 session 中间件。
func New() *gin.Engine {
	r := gin.Default()

	store := cookie.NewStore([]byte("filestore-session-secret"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false,
	})
	r.Use(sessions.Sessions("filestore_session", store))

	r.POST("/user/signup", api.Signup)
	r.POST("/user/login", api.Login)
	r.POST("/user/logout", api.Logout)

	auth := r.Group("/")
	auth.Use(mw.AuthMiddleware())
	auth.GET("/file/upload", api.UploadFile)
	auth.POST("/file/upload", mw.RequireUploadFile("file"), api.UploadFile)
	auth.GET("/file/meta", mw.RequireFileHash(), api.GetFileMeta)
	auth.GET("/file/download", mw.RequireFileHash(), api.DownloadFile)
	auth.POST("/file/update", mw.RequireFileHash(), mw.RequireOp("0"), mw.RequireFilename(), api.FileMetaUpdate)
	auth.POST("/file/delete", mw.RequireFileHash(), api.FileDelete)
	auth.POST("/user/filelist",mw.RequireUsername(),api.UserFilelistQuery)
	return r
}
