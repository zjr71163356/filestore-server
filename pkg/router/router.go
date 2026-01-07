package router

import (
	"filestore-server/api"
	"filestore-server/config"
	"filestore-server/pkg/mw"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// New 构建 gin.Engine，注册路由与 session 中间件。
func New(cfg config.Config) *gin.Engine {
	r := gin.Default()

	secret := cfg.Session.Secret
	if secret == "" {
		secret = "filestore-session-secret"
	}
	sessionName := cfg.Session.Name
	if sessionName == "" {
		sessionName = "filestore_session"
	}
	maxAge := cfg.Session.MaxAge
	if maxAge <= 0 {
		maxAge = 86400 * 7
	}

	store := cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.Session.Secure,
	})
	r.Use(sessions.Sessions(sessionName, store))

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
	auth.POST("/user/filelist", mw.RequireUsername(), api.UserFilelistQuery)
	auth.POST("/mpfile/init", mw.RequireUploadInitMutipart(), api.UploadInitMutipart)
	auth.POST("/mpfile/upload", mw.RequireUploadPart(), api.UploadPartHandler)
	auth.POST("/mpfile/complete", mw.RequireUploadComplete(), api.CompleteUploadHandler)
	return r
}
