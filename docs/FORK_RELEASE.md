# Fork 发版流程

本文档记录此 new-api fork 分支的完整发版流程。

## 基本信息

- **Fork 仓库**: `https://github.com/3166790565/new-api.git`
- **GitHub 账号**: `3166790565`
- **Git 用户**: 白
- **Docker Hub 账号**: `bxacc123`
- **服务器路径**: `/opt/new-api`
- **当前版本**: v1.0.2

## 重要说明

### VERSION 文件格式
- `VERSION` 文件内容**必须包含 `v` 前缀**
- 示例: 文件内容为 `v1.0.2`(末尾带一个换行符)

### Git 提交信息
- 在 Bash 工具中使用 heredoc 语法: `git commit -F - <<'EOF' ... EOF`
- **不要**使用 PowerShell here-string 语法 `@'...'@`(Bash 工具是 Git Bash/POSIX sh,`@` 会变成字面字符)

### Docker 镜像构建
- 服务器架构: x86_64 Linux
- 构建时必须指定: `--platform linux/amd64`
- 镜像推送到: `bxacc123/new-api:<version>` 和 `bxacc123/new-api:latest`

### 版本注入机制
- `VERSION` 文件在构建时通过 Dockerfile ldflags 注入到 `common.Version`
- 默认值(当 VERSION 为空时): `v0.0.0`
- 系统更新检查已重定向到此 fork: `web/src/features/system-update/api.ts` 和 `releases.ts`
- 检查地址: `api.github.com/repos/3166790565/new-api/releases`

### 受保护的项目信息
**严禁修改以下内容**(按 CLAUDE.md 项目治理规则):
- Go 模块路径 `github.com/QuantumNous/new-api`
- 任何 QuantumNous 品牌/归属信息
- new-api 项目名称和身份标识

## 完整发版流程

### 1. 准备新版本

```bash
# 更新 VERSION 文件(内容包含 v 前缀)
echo "v1.0.3" > VERSION

# 提交版本变更
git add VERSION
git commit -F - <<'EOF'
chore(release): 设定 fork 版本号为 v1.0.3

新版本功能说明...
EOF

# 推送到 main 分支
git push origin main
```

### 2. 创建 Git 标签

```bash
# 创建带注释的标签
git tag -a v1.0.3 -m "Release v1.0.3"

# 推送标签
git push origin v1.0.3
```

### 3. 构建 Docker 镜像

```bash
# 构建镜像(同时打版本标签和 latest 标签)
docker build --platform linux/amd64 \
  -t bxacc123/new-api:v1.0.3 \
  -t bxacc123/new-api:latest \
  .

# 推送镜像
docker push bxacc123/new-api:v1.0.3
docker push bxacc123/new-api:latest
```

### 4. 创建 GitHub Release

```bash
# 使用 gh CLI 创建 Release(需要已认证为 3166790565 账号)
gh release create v1.0.3 \
  --repo 3166790565/new-api \
  --title "v1.0.3" \
  --notes "发版说明内容..."
```

### 5. 服务器部署

**由用户在服务器上执行**:

```bash
cd /opt/new-api

# 拉取最新代码
git pull

# 拉取最新镜像
docker compose pull new-api

# 重启服务
docker compose up -d
```

## 服务器配置说明

### Docker Compose 配置
- `docker-compose.yml` 使用 `image: bxacc123/new-api:latest`
- `build:` 块已注释(作为后备方案保留)
- 原因: 服务器曾出现 DNS/网络故障,无法访问 `deb.debian.org` 进行源码构建

### 数据持久化
- PostgreSQL 数据卷: `new-api_pg_data`
- 绑定挂载: `./data`
- 只要目录保持在 `/opt/new-api` 且卷名不变,数据在镜像更新时会保留

## Force Push 注意事项

- Claude Code auto-mode 的权限分类器会阻止 force push 到 main,即使对话中已授权
- 如需重写历史,用户必须手动在提示符中运行:
  ```
  ! git push --force-with-lease origin main
  ```
- 普通 `git push` 和标签推送不受影响

## 版本历史

- **v1.0.2** (2026-09-25): 修复 OAuth 登录错误要求注册码 + 新增独立 `/oauth-register-code` 页面
- **v1.0.1** (2026-09-24): 管理员统计看板(日活/站点统计/IP 地区分布)
- **v1.0.0** (2026-09-23): 首个 fork 版本

## 相关文档

- 渠道贡献功能: 参见相关功能分支文档
- 统计看板功能: 参见 `statistics-dashboard-feature` 相关文档
