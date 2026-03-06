# 🧭 Navi — 个人导航页

部署在个人服务器上的导航首页，支持 Cloudflare D1 云备份。  
单一可执行文件，内嵌前端，下载即用，通过 Cloudflare Tunnel 对外暴露。

---

## ⚡ 一键安装（Linux 服务器）

```bash
curl -fsSL https://raw.githubusercontent.com/fengzhanhuaer/Navi/main/scripts/install.sh | sudo bash
```

> 脚本会自动：检测平台 → 下载最新二进制 → 生成配置 → 注册 systemd 服务并启动。

安装完成后访问：`http://YOUR_SERVER_IP:15020`

---

## 功能
- 🔍 多搜索引擎快速切换（Google / 百度 / Bing / DuckDuckGo）
- ⭐ 分组书签管理，支持折叠/展开
- ☁️ Cloudflare D1 自动云备份（每5分钟）
- 🌗 深色/浅色主题切换
- ⌨️ 快捷键 `/` 聚焦搜索框

## 技术栈
- **后端**: Go + Gin + SQLite（modernc/sqlite，纯 Go）
- **前端**: 原生 HTML + CSS + JavaScript（无框架，嵌入二进制）
- **云备份**: Cloudflare D1 via REST API

---

## 本地开发

```bash
git clone https://github.com/fengzhanhuaer/Navi.git
cd Navi
cp .env.example .env       # 可选：填写 D1 配置
go mod tidy
FRONTEND_DIR=./frontend go run .
```

浏览器访问 http://localhost:15020

---

## 配置 Cloudflare D1（可选，云备份）

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 进入 **Workers & Pages → D1** → 新建数据库
3. 记录 `Account ID` 和 `Database ID`
4. 创建 API Token（权限：`D1 Edit`）
5. 编辑 `/opt/navi/.env`：

```env
CF_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
CF_D1_DATABASE_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
CF_API_TOKEN=your_token_here
```

```bash
sudo systemctl restart navi
```

重启后数据每 5 分钟自动同步到 D1，也可在设置页手动触发。

---

## 发布新版本

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions 自动构建 5 个平台的二进制并创建 Release，安装脚本会自动下载最新版。

---

## 手动管理服务

```bash
sudo systemctl status  navi   # 查看状态
sudo systemctl restart navi   # 重启
sudo systemctl stop    navi   # 停止
sudo journalctl -u navi -f    # 实时日志
```

---

## API 文档

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/data | 一次性获取全部初始化数据 |
| GET | /api/groups | 获取所有分组 |
| POST | /api/groups | 创建分组 |
| PUT | /api/groups/:id | 更新分组 |
| DELETE | /api/groups/:id | 删除分组 |
| GET | /api/sites | 获取所有网站 |
| POST | /api/sites | 添加网站 |
| PUT | /api/sites/:id | 更新网站 |
| DELETE | /api/sites/:id | 删除网站 |
| GET | /api/settings | 获取配置 |
| PUT | /api/settings/:key | 更新配置 |
| POST | /api/sync/push | 手动推送到 D1 |
| POST | /api/sync/restore | 从 D1 恢复本地 |
| GET | /api/sync/status | D1 连接状态 |
| GET | /api/sync/logs | 同步日志 |
| GET | /health | 健康检查 |
