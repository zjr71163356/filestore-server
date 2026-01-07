### 流程
客户端发起请求：GET/POST http://localhost:8080/file/upload
HTTP 服务器匹配路由路径 /file/upload
调用 handler.UploadFileHandler 函数来处理请求

### 开发日志
- 2025.12.10 feat:实现了上传功能。创建了main.go文件，本进程监听localhost:8080，客户端发起请求：GET/POST http://localhost:8080/file/upload时HTTP服务器会进行路径匹配，会调用对应的函数handler处理请求
- 2025.12.11 feat:实现了下载文件、查询文件、修改文件元信息的操作，并添加了端到端测试和覆盖测试，全部通过测试
- 2025.12.11 env:实现了mysql主从模式，了解了主从、单点、多主模式，分析本项目更适合用哪种模式及其原因

### 配置
- 配置集中在 `config/config.go`，默认使用内置值，启动时优先加载 `.env`（或 `ENV_FILE` 指定路径），然后再读取已存在的环境变量进行覆盖；已有环境变量优先于 `.env`。
- 支持的环境变量：`SERVER_ADDR`、`DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_MAX_OPEN_CONNS`、`REDIS_ADDR/REDIS_PASSWORD`、`SESSION_NAME/SESSION_SECRET/SESSION_MAX_AGE/SESSION_SECURE`、`STORAGE_TMP_DIR`。
- 示例文件：`.env.example`，复制为 `.env` 后按环境填写，可同时被应用和 docker-compose 使用；`env/docker-compose.yml` 已引用 `../.env`，端口/数据库名/密码会与应用保持一致。
