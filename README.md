### 流程
客户端发起请求：GET/POST http://localhost:8080/file/upload
HTTP 服务器匹配路由路径 /file/upload
调用 handler.UploadFileHandler 函数来处理请求

### 开发日志
- 2025.12.10 feat:实现了上传功能。创建了main.go文件，本进程监听localhost:8080，客户端发起请求：GET/POST http://localhost:8080/file/upload时HTTP服务器会进行路径匹配，会调用对应的函数handler处理请求
- 2025.12.11 feat:实现了下载文件、查询文件、修改文件元信息的操作，并添加了端到端测试和覆盖测试，全部通过测试
- 2025.12.11 env:实现了mysql主从模式，了解了主从、单点、多主模式，分析本项目更适合用哪种模式及其原因
