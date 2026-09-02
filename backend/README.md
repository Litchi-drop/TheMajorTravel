# MajorTravel 后端（Go + Gin）

协作版后端，技术栈与设计见 `../docs/项目开发报告.md`（v4.0）。

## 目录结构

```
backend/
├── cmd/server/          # 入口：main.go
├── internal/
│   ├── config/          # 环境变量加载（.env 仅本地）
│   ├── handler/         # 路由与请求/响应（统一 {ok,data|error} 响应包）
│   ├── middleware/      # JWT 鉴权、限流（Step 1）
│   ├── service/         # 业务逻辑（Step 1：验证码/diff）
│   └── repository/      # GORM 数据访问（Step 1）
├── seed/                # 蓝本种子 JSON（Step 2）
├── scripts/             # 构建/运维脚本（Step 4：build.ps1 单二进制构建）
├── docker-compose.yml   # 本地 Redis 7（PostgreSQL 用原生 Windows 服务，不在容器里）
├── .env.example         # 环境变量样例（复制为 .env 使用）
└── go.mod
```

## 本地启动

```bash
# 1. PostgreSQL：原生 Windows 服务 postgresql-x64-16（开机自启），开发库 major_travel
#    连接用 postgres 账号，密码只写本机 .env（已 gitignore，绝不进仓库）

# 2. Redis：Docker 容器
docker compose up -d

# 3. 准备环境变量
cp .env.example .env    # Windows CMD 用 copy

# 4. 运行
go run ./cmd/server

# 5. 健康检查
curl http://localhost:8080/api/health
```

## GoLand 首次打开

1. File → Open 选择本 `backend` 目录（作为独立项目打开，GoLand 会识别 go.mod）；
2. 提示下载 Go SDK 时选择最新稳定版（≥ 1.23）；
3. 若依赖下载超时：Settings → Go → Go Modules，把 Proxy 改为
   `https://goproxy.cn,direct`（国内直连 proxy.golang.org 不可达）。

## 依赖清单（随 Step 推进逐步引入）

| 用途 | 包 | 引入 |
|---|---|---|
| Web 框架 | github.com/gin-gonic/gin | ✅ Step 0 |
| .env 加载 | github.com/joho/godotenv | ✅ Step 0 |
| ORM (PG) | gorm.io/gorm + gorm.io/driver/postgres | Step 1 |
| Redis | github.com/redis/go-redis/v9 | Step 1 |
| JWT | github.com/golang-jwt/jwt/v5 | Step 1 |
| 密码哈希 | golang.org/x/crypto/bcrypt | Step 1 |
| 邮箱验证码 | net/smtp（Go 标准库） | Step 1 |
