# AGENTS.md

macOS LaunchAgents 管理面板：单页面 Web UI，查看并管理 `gui/$UID` 域的 launchctl 服务。前后端分离：Go 纯 JSON API 后端 + React/Vite/TypeScript 前端（pnpm）。

## 常用命令

```bash
# 后端
go build ./...                    # 编译
go vet ./...                      # 静态检查
go build -o localctl ./cmd/localctl   # 构建可执行文件
./localctl                        # 默认监听 127.0.0.1:7788（纯 JSON API）

# 前端（frontend/ 目录下）
pnpm install                      # 安装依赖
pnpm dev                          # Vite dev server，http://localhost:5173，/api 代理到 127.0.0.1:7788
pnpm build                        # tsc 类型检查 + 构建产物到 dist/
```

开发时需同时跑后端（go run ./cmd/localctl）与前端（pnpm dev）。前端构建产物 dist/ 独立部署，Go 不托管静态文件。

## 配置

支持 `.env`（参考 `.env.example`，`.env` 已 gitignore）与 `~/.localctl/config.json`，优先级：进程环境变量 > `.env` > config.json > 首次启动自动生成随机密码。变量：

- `LOCALCTL_ADDR`：监听地址（默认 127.0.0.1:7788）
- `LOCALCTL_PASSWORD`：登录密码（明文，启动时内存中做 bcrypt 哈希，不落盘）
- `LOCALCTL_SECRET`：token 签名 HMAC 密钥
- `LOCALCTL_CORS_ORIGIN`：允许的跨域来源（默认 `http://localhost:5173`，仅前后端分域部署时需要）

`.env` 相对进程工作目录加载。

## 目录结构

```
cmd/localctl/main.go        # 入口：加载 .env、配置、启动 API server
internal/launchd/
  launchctl.go              # /bin/launchctl exec 封装（10s 超时）+ print/print-disabled 输出解析
  actions.go                # Service 聚合模型 + Start/Stop/Restart/Enable/Disable/Load/Unload
internal/plistinfo/parse.go # 扫描 ~/Library/LaunchAgents/*.plist，howett.net/plist 解析
internal/server/
  server.go                 # 路由注册、CORS 中间件
  auth.go                   # HMAC 签名 bearer token（Authorization: Bearer，7 天有效）
  handlers.go               # JSON handlers + Service JSON 视图模型
internal/config/config.go   # .env 加载 + ~/.localctl/config.json + 密码校验
frontend/                   # React 19 + Vite 7 + TypeScript（pnpm）
  src/api.ts                # fetch 客户端：token 管理、API 封装、401 统一处理
  src/types.ts              # 与后端 JSON 对齐的类型
  src/hooks/useServices.ts  # 5 秒轮询 hook
  src/components/           # StatusBadge / ServiceCard / DetailPanel
  src/pages/                # Login / Dashboard
```

## API（全部 JSON，前缀 /api）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /api/health | 无需认证，健康检查 |
| POST | /api/auth/login | `{password}` → `{token, expires_at}` |
| GET | /api/auth/me | 校验 token |
| GET | /api/services | 服务列表（前端每 5 秒轮询） |
| GET | /api/services/{label}?path= | 单服务详情（含 plist 配置 `agent`） |
| POST | /api/services/{label}/actions | `{op, path?}` → `{service}`，op ∈ start/restart/stop/enable/disable/load/unload |

认证：除 /api/health 外均需 `Authorization: Bearer <token>`；无效返回 401 + `{"error": "..."}`。API 设计对 MCP 等外部客户端友好。

## 架构与数据流

- 前端每 5 秒 fetch `GET /api/services` 拿 JSON 全量刷新；操作按钮 POST actions 后返回刷新后的 service JSON 并立即触发一次刷新；详情懒加载 `GET /api/services/{label}`。
- 列表来源：**只显示 `~/Library/LaunchAgents` 下有 plist 文件的 agent**（plist 视角，有意排除 `launchctl list` 里的 com.apple.* 噪音）。
- 单个服务状态：先 `launchctl print gui/$UID/<label>` 拿运行时信息；失败则回退 `launchctl print-disabled` 判断 enabled。
- 前端 401 统一处理：api.ts 清除 token 并派发 `localctl:unauthorized` 事件，App.tsx 监听后回到登录页。

## launchctl 本机已知行为（重要）

1. 本机 macOS 的 `launchctl print` **不支持 `-json`**，输出为旧式文本块（`key = value`，嵌套 `{...}`）。解析逻辑在 `parseBlock`，顶层块的起始行是 `<target> = {`，解析需从下一行开始（`idx := i + 1`）。
2. `print` 输出中程序参数块叫 `arguments`（不是 program arguments），成员是不带引号的裸行。
3. 被 disable / 未 bootstrap 的服务 `launchctl print` 报 "Could not find service ... in domain"，属正常回退路径，不是错误。
4. `launchctl print-disabled gui/$UID` 输出 `"label" => disabled|enabled` 行格式。

## 安全模型

- 后端仅监听 127.0.0.1；bcrypt 密码 + HMAC 签名 bearer token（7 天），无 CSRF 面（不用 cookie）。
- 密码哈希与 secret key 明文存于 `~/.localctl/config.json`（0600）或 `.env`，勿在代码或日志中输出其内容。

## 约定

- 用户域固定 `gui/$UID`，不支持 system 域（管理 LaunchDaemons 需要 sudo，超出本项目范围）。
- 管理操作直接生效于 launchctl；Stop 用 `kill SIGTERM`，Disable 是持久化的（重启后仍禁用）。
- 危险操作（Stop/Disable/Unload）前端用 `window.confirm` 二次确认（见 ServiceCard.tsx 的 DANGEROUS_OPS），新增操作请保持此约定。
- 代码注释与 UI 文案混用中英文；前端用户可见文案为中文。
- 前端类型必须与后端 JSON 对齐：改后端字段时同步 `frontend/src/types.ts`。
