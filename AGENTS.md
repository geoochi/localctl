# AGENTS.md

macOS LaunchAgents 管理面板：单页面 Web UI，查看并管理 `gui/$UID` 域的 launchctl 服务。Go 后端 + React/Vite/TypeScript 前端（pnpm）；前端构建产物经 go:embed 打进单个二进制，单进程部署。

## 常用命令

```bash
make build                        # 前端 pnpm build + Go 编译（产物 ./localctl 单二进制）
make clean                        # 清理
go build ./... && go vet ./...    # 仅后端编译与静态检查（需先有 frontend/dist，否则 embed 失败）
./localctl                        # 监听 127.0.0.1:8003，同时服务页面与 /api
./localctl -addr 127.0.0.1:9000   # 自定义端口

# 前端开发模式（可选，改前端时用）
cd frontend && pnpm install && pnpm dev   # Vite :8003，/api 代理到 127.0.0.1:7788
# 此时后端需另跑在 7788：LOCALCTL_ADDR=127.0.0.1:7788 go run ./cmd/localctl
```

注意：`frontend/dist` 已 gitignore 但被 go:embed 引用，clone 后必须先 `make frontend`（或 cd frontend && pnpm build）再编译 Go。

## 配置

.env 是唯一配置来源（参考 `.env.example`，`.env` 已 gitignore），优先级：进程环境变量 > `.env`。`.env` 从工作目录加载，找不到时回退到可执行文件所在目录。变量：

- `LOCALCTL_ADDR`：监听地址（默认 127.0.0.1:8003）
- `LOCALCTL_CORS_ORIGIN`：允许的跨域来源（默认不启用；内嵌前端同源无需 CORS，仅外部 API 客户端需要时设置）

`.env` 相对进程工作目录加载。

## 目录结构

```
cmd/localctl/main.go        # 入口：加载 .env、解析监听地址、启动 server
internal/launchd/
  launchctl.go              # /bin/launchctl exec 封装（10s 超时）+ print/print-disabled 输出解析
  actions.go                # Service 聚合模型 + Start/Stop/Restart/Enable/Disable/Load/Unload
internal/plistinfo/parse.go # 扫描 ~/Library/LaunchAgents/*.plist，howett.net/plist 解析
internal/server/
  server.go                 # 路由注册、CORS 中间件（opt-in）、内嵌前端 SPA 托管
  auth.go                   # HMAC 签名 bearer token（Authorization: Bearer，7 天有效）
  handlers.go               # JSON handlers + Service JSON 视图模型
frontend/
  embed.go                  # go:embed dist（需先 pnpm build）
internal/config/config.go   # .env 加载 + ~/.localctl/config.json + 密码校验
frontend/                   # React 19 + Vite 7 + TypeScript（pnpm）
  src/api.ts                # fetch 客户端：API 封装、错误处理
  src/types.ts              # 与后端 JSON 对齐的类型
  src/hooks/useServices.ts  # 5 秒轮询 hook
  src/components/           # StatusBadge / ServiceCard / DetailPanel
  src/pages/                # Dashboard
```

## API（全部 JSON，前缀 /api）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /api/health | 健康检查 |
| GET | /api/services | 服务列表（前端每 5 秒轮询） |

除 /api 外的所有 GET 路径由内嵌前端托管（SPA fallback 到 index.html）。
| GET | /api/services/{label}?path= | 单服务详情（含 plist 配置 `agent`，`agent.run_description` 为运行方式中文摘要） |
| GET | /api/services/{label}/source?path=&download= | plist 原始 XML 内容（download=1 时作为附件下载） |
| POST | /api/services/{label}/source?path= | 保存 plist 源文件：`{content}`，校验格式后写入，若服务已加载则自动重载 |
| POST | /api/services/{label}/actions | `{op, path?}` → `{service}`，op ∈ start/restart/stop/enable/disable/load/unload/delete |
| POST | /api/plist | 新建 LaunchAgent：`{label, command(shell), type: runatload/interval/calendar, interval_seconds?/hour?+minute?+weekdays?, keep_alive?, working_dir?/std_out_path?/std_err_path?}` → 201 `{service}` |
| GET | /api/cron | 解析用户 crontab，标注每条是否可导入 launchd |
| POST | /api/cron/{index}/import | 将一条 cron 条目导入为 LaunchAgent（写 plist → bootstrap → 从 crontab 移除原行） |

无认证（仅监听 127.0.0.1，本机使用）；错误统一返回 HTTP 状态码 + `{"error": "..."}`。API 设计对 MCP 等外部客户端友好。

## 架构与数据流

- 单进程：Go 在 8003 同时服务内嵌前端页面与 /api。前端每 5 秒 fetch `GET /api/services` 拿 JSON 全量刷新；操作按钮 POST actions 后返回刷新后的 service JSON 并立即触发一次刷新；详情懒加载 `GET /api/services/{label}`。
- 列表来源：**只显示 `~/Library/LaunchAgents` 下有 plist 文件的 agent**（plist 视角，有意排除 `launchctl list` 里的 com.apple.* 噪音）。
- 单个服务状态：先 `launchctl print gui/$UID/<label>` 拿运行时信息；失败则回退 `launchctl print-disabled` 判断 enabled。
- 操作后除返回刷新的 service JSON 外，前端还会立即触发一次列表刷新。
- Delete（op=delete）：bootout 后删除 plist 文件，仅允许 `~/Library/LaunchAgents` 下的路径（后端强校验），前端二次确认。
- Cron 导入（internal/cron）：数字/区间/步进/逗号列表展开为 StartCalendarInterval；`*/n * * * *` 用 StartInterval n*60 近似（有漂移）；日+星期同时受限不可导入（cron 或语义 vs launchd 且语义）；导入时同步从 crontab 移除原行避免双重执行。

## launchctl 本机已知行为（重要）

1. 本机 macOS 的 `launchctl print` **不支持 `-json`**，输出为旧式文本块（`key = value`，嵌套 `{...}`）。解析逻辑在 `parseBlock`，顶层块的起始行是 `<target> = {`，解析需从下一行开始（`idx := i + 1`）。
2. `print` 输出中程序参数块叫 `arguments`（不是 program arguments），成员是不带引号的裸行。
3. 被 disable / 未 bootstrap 的服务 `launchctl print` 报 "Could not find service ... in domain"，属正常回退路径，不是错误。
4. `launchctl print-disabled gui/$UID` 输出 `"label" => disabled|enabled` 行格式。

## 安全模型

- 仅监听 127.0.0.1，无认证模块（本机使用）；页面与 API 同源，默认无 CORS。

## 约定

- 用户域固定 `gui/$UID`，不支持 system 域（管理 LaunchDaemons 需要 sudo，超出本项目范围）。
- 管理操作直接生效于 launchctl；Stop 统一用 bootout（不用 kill —— 直接杀进程与 launchd 抢管理权，KeepAlive 服务还会被立即拉起），Stop 后服务未加载，恢复用 Load/Start；Disable 是持久化的（重启后仍禁用）。
- 危险操作（Stop/Disable/Unload）前端用 `window.confirm` 二次确认（见 ServiceCard.tsx 的 DANGEROUS_OPS），新增操作请保持此约定。
- 代码注释与 UI 文案混用中英文；前端用户可见文案为中文。
- 前端类型必须与后端 JSON 对齐：改后端字段时同步 `frontend/src/types.ts`。
