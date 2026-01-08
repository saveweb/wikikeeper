# WikiKeeper 开发环境说明

## 🚀 快速开始

### 方式一：全部在 Docker 中运行（推荐）

最简单的方式，一键启动所有服务：

```bash
make dev-docker
```

这会启动：
- PostgreSQL 数据库（localhost:5432）
- 后端 API（localhost:8000）- 支持热重载
- 前端（localhost:5173）- 支持热重载

然后运行数据库迁移：
```bash
make db-migrate
```

访问：
- 前端: http://localhost:5173
- 后端: http://localhost:8000

### 方式二：数据库在 Docker，应用在本地（推荐用于调试）

```bash
# 1. 启动数据库
make dev-local-db

# 2. 运行迁移
make db-migrate

# 3. 在不同终端启动应用
# Terminal 1: 后端（支持热重载）
make run-dev

# Terminal 2: 前端
make frontend-dev
```

## 📋 常用命令

### 🐳 Docker 开发（全部在 Docker）

```bash
make dev-docker           # 启动所有服务
make dev-docker-down      # 停止所有服务
make dev-docker-logs      # 查看日志
make dev-docker-tools     # 启动服务 + Adminer (DB UI at :8080)
```

### 💻 本地开发（数据库在 Docker）

```bash
make dev-local-db         # 启动数据库
make dev-local-down       # 停止数据库
make run                  # 运行后端
make run-dev              # 运行后端（热重载）
make frontend-dev         # 运行前端
```

### 🗄️ 数据库操作

```bash
make db-migrate           # 运行迁移
make db-reset             # 重置数据库
make db-shell             # 打开 PostgreSQL shell
make db-dump FILE=f.sql   # 备份数据库
```

### 🧪 测试

```bash
make test                 # 运行测试 (short mode)
make test-all             # 运行所有测试包括集成测试
make test-coverage        # 生成测试覆盖率报告
```

### ✨ 代码质量

```bash
make fmt                  # 格式化代码
make lint                 # 运行 linter
make staticcheck          # 运行 staticcheck
make check                # 运行所有检查 (fmt + lint + test)
```

### 🧹 清理

```bash
make clean                # 清理构建产物
make clean-all            # 清理所有包括 Docker volumes
```

## 🛠️ 开发工具

```bash
make install-tools        # 安装所有开发工具
```

工具列表：
- `golangci-lint`: Go linter
- `staticcheck`: Static analysis
- `air`: Live reload for Go apps
- `migrate`: Database migration tool

## 🔧 热重载开发

### 后端热重载

使用 `air` 进行热重载：

```bash
make run-dev
```

修改代码后会自动重新编译和重启服务。air 配置文件：`.air.toml`

支持的特性：
- ✅ 彩色输出
- ✅ 显示时间戳
- ✅ 自动重启
- ✅ 排除测试文件

### 前端热重载

```bash
make frontend-dev
```

使用 Vite 的 HMR (Hot Module Replacement)。

## 🗄️ 数据库管理

### 使用 Adminer (Web UI)

```bash
make dev-docker-tools
```

访问：http://localhost:8080

连接信息：
- System: PostgreSQL
- Server: postgres
- Username: wikikeeper
- Password: wikikeeper123
- Database: wikikeeper

### 使用命令行

```bash
make db-shell
```

## 🌐 环境配置

开发环境默认配置：

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=wikikeeper
DB_PASSWORD=wikikeeper123
DB_NAME=wikikeeper

# Server
HOST=0.0.0.0
PORT=8000

# Debug
DEBUG=true
LOG_LEVEL=DEBUG

# HTTP Client
HTTP_TIMEOUT=30.0
HTTP_USER_AGENT=WikiKeeper-Dev/0.2.0
```

可以在 `.env` 文件中覆盖这些配置。

## 🧪 测试策略

### 单元测试（不需要数据库）

```bash
make test
```

使用 SQLite 内存数据库，快速运行。

### 集成测试（需要 PostgreSQL）

```bash
# 1. 启动数据库
make dev-local-db
make db-migrate

# 2. 运行测试
make test-all
```

使用真实的 PostgreSQL 数据库。

## 📁 项目结构

```
backend/
├── cmd/
│   └── server/
│       └── main.go          # 应用入口
├── internal/
│   ├── config/              # 配置管理
│   ├── database/            # 数据库连接
│   ├── handlers/            # HTTP handlers
│   ├── logger/              # 日志
│   ├── middleware/          # 中间件
│   ├── models/              # 数据模型
│   ├── repository/          # 数据访问层
│   └── services/            # 业务逻辑
├── migrations/              # 数据库迁移
├── .air.toml                # Air 配置 (热重载)
├── Dockerfile.dev           # 开发用 Dockerfile
├── Dockerfile               # 生产用 Dockerfile
├── Makefile                 # Make 命令
├── dev-setup.sh             # 开发环境设置脚本
└── docker-compose.dev.yml   # 开发环境配置
```

## 🔍 故障排查

### 端口冲突

如果端口被占用：

1. **5432 端口 (PostgreSQL)**: 修改 `docker-compose.dev.yml` 中的端口映射
2. **8000 端口 (Backend)**: 设置环境变量 `PORT=8001 make run`
3. **5173 端口 (Frontend)**: 修改 `frontend/package.json` 中的 dev 脚本

### 数据库连接失败

检查数据库是否运行：

```bash
docker ps | grep wikikeeper-postgres-dev
```

查看数据库日志：

```bash
make dev-db-logs
```

### 清理环境

完全清理（包括 Docker volumes）：

```bash
make clean-all
```

## 🚀 生产部署

生产环境使用根目录的 `docker-compose.yml`：

```bash
cd ..
docker-compose up -d
```

访问：http://127.0.0.1:8732

## 📝 开发工作流

### 典型开发流程

1. **启动开发环境**
   ```bash
   make dev-docker
   make db-migrate
   ```

2. **开发代码**
   - 修改后端代码（自动重新加载）
   - 修改前端代码（自动 HMR）

3. **运行测试**
   ```bash
   make test
   ```

4. **代码检查**
   ```bash
   make check
   ```

5. **提交代码**
   ```bash
   git add .
   git commit -m "描述你的更改"
   ```

### 添加新功能

1. 创建新分支
   ```bash
   git checkout -b feature/new-feature
   ```

2. 开发并测试
   ```bash
   make run-dev
   make test
   ```

3. 代码检查
   ```bash
   make fmt
   make lint
   make staticcheck
   ```

4. 提交
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

## 📚 相关文档

- [Go 测试最佳实践](https://golang.org/doc/effective_go.html#testing)
- [Air 热重载工具](https://github.com/cosmtrek/air)
- [GORM 文档](https://gorm.io/docs/)
- [SvelteKit 文档](https://kit.svelte.dev/)
