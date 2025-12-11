### 流程
客户端发起请求：GET/POST http://localhost:8080/file/upload
HTTP 服务器匹配路由路径 /file/upload
调用 handler.UploadFileHandler 函数来处理请求

### 开发日志
- 2025.12.10 feat:实现了上传功能。创建了main.go文件，本进程监听localhost:8080，客户端发起请求：GET/POST http://localhost:8080/file/upload时HTTP服务器会进行路径匹配，会调用对应的函数handler处理请求