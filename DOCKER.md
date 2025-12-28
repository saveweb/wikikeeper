# WikiKeeper Docker 部署文档

## 快速开始

### 1. 准备环境变量

复制并配置环境变量：

```bash
cp .env.example .env
```

编辑 `.env` 文件，根据需要修改配置：

```bash
# 必须修改的配置
POSTGRES_PASSWORD=your_secure_password
ADMIN_TOKEN=your_admin_token  # 可选，用于管理功能
```

### 2. 启动服务

```bash
# 构建并启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f

# 查看服务状态
docker compose ps
```

### 3. 访问服务

- **Backend API**: http://localhost:8000
- **API 文档**: http://localhost:8000/docs (如果启用)
- **PostgreSQL**: localhost:5432

## 服务说明

### PostgreSQL (postgres)
- **端口**: 5432
- **数据库**: wikikeeper
- **用户**: wikikeeper
- **密码**: 由 `POSTGRES_PASSWORD` 环境变量指定
- **数据持久化**: Docker volume `postgres_data`
- **自动初始化**: 首次启动时会导入 `wikikeeper_backup.sql`（如果存在）

### Backend (backend)
- **端口**: 8000
- **依赖**: PostgreSQL (通过 healthcheck 确保就绪后才启动)
- **自动重启**: `unless-stopped`

## 常用命令

```bash
# 停止所有服务
docker compose down

# 停止并删除所有数据（包括数据库）
docker compose down -v

# 重新构建镜像
docker compose build

# 重启服务
docker compose restart

# 查看特定服务日志
docker compose logs -f backend
docker compose logs -f postgres

# 进入运行中的容器
docker compose exec backend sh
docker compose exec postgres psql -U wikikeeper wikikeeper
```

## 数据备份

### 导出数据

```bash
# 从运行中的容器导出 SQL
docker exec wikikeeper-postgres pg_dump -U wikikeeper wikikeeper > wikikeeper_backup_$(date +%Y%m%d).sql

# 或者使用 docker compose
docker compose exec postgres pg_dump -U wikikeeper wikikeeper > wikikeeper_backup_$(date +%Y%m%d).sql
```

### 导入数据

将 SQL 文件放在项目根目录并命名为 `wikikeeper_backup.sql`，然后启动服务：

```bash
docker compose up -d
```

初始化脚本会自动导入数据。

## 生产环境建议

1. **修改默认密码**: 务必修改 `POSTGRES_PASSWORD`
2. **设置 ADMIN_TOKEN**: 启用管理功能以保护敏感操作
3. **配置资源限制**: 在 docker-compose.yml 中添加资源限制
4. **使用 secrets**: 使用 Docker secrets 管理敏感信息
5. **配置日志轮转**: 防止日志文件过大
6. **定期备份**: 设置定期备份任务

## 资源限制示例

在 `docker-compose.yml` 中添加：

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M

  postgres:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
```

## 故障排查

### 数据库连接失败

```bash
# 检查 PostgreSQL 状态
docker compose ps postgres
docker compose logs postgres

# 测试数据库连接
docker compose exec postgres pg_isready -U wikikeeper
```

### Backend 无法启动

```bash
# 查看后端日志
docker compose logs backend

# 检查环境变量
docker compose exec backend env | grep DB_

# 手动测试
docker compose exec backend sh
./server
```

### 重新初始化数据库

```bash
# 停止服务
docker compose down

# 删除数据卷
docker volume rm wikikeeper_postgres_data

# 重新启动
docker compose up -d
```
