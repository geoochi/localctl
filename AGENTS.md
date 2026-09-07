# AGENTS.md

macOS LaunchAgents 管理面板：单页面 Web UI，查看并管理 `gui/$UID` 域的 launchctl 服务。Go 全栈，无 Node 构建链，所有资源 `go:embed` 进单个二进制。

## 常用命令

```bash
go build ./...                    # 编译
go vet ./...                      # 静态检查
go build -o localctl ./cmd/localctl   # 构建可执行文件
./localctl                        # 默认监听 127.0.0.1:7788
./localctl -addr 127.0.0.1:9000   # 自定义端口
```

## 配置

支持 `.env`（参考 `.env.example`，`.env` 已 gitignore）与 `~/.localctl/config.json`，优先级：进程环境变量 > `.env` > config.json > 首次启动自动生成随机密码。变量：`LOCALCTL_ADDR`、`LOCALCTL_PASSWORD`（明文，启动时内存中做 bcrypt 哈希，不落盘）、`LOCALCTL_SECRET`。`.env` 相对进程工作目录加载。

## 目录结构

```
cmd/localctl/main.go        # 入口：加载配置、启动 server
internal/launchd/
  launchctl.go              # /bin/launchctl exec 封装（10s 超时）+ print/print-disabled 输出解析
  actions.go                # Service 聚合模型 + Start/Stop/Restart/Enable/Disable/Load/Unload
internal/plistinfo/parse.go # 扫描 ~/Library/LaunchAgents/*.plist，howett.net/plist 解析
internal/server/
  server.go                 # 路由注册、模板加载、静态资源
  auth.go                   # HMAC 签名 session cookie（HttpOnly + SameSite=Lax）
  handlers.go               # 页面 / partial / action handlers，视图模型 ServiceRow
internal/config/config.go   # ~/.localctl/config.json（bcrypt 密码哈希、secret key、监听地址）
web/
  embed.go                  # go:embed templates/ static/
  templates/*.html          # index/login/services/service_row/service_detail
  static/htmx.min.js
```

## 架构与数据流

- 前端：Go `html/template` + htmx。列表容器 `#service-list` 每 5 秒轮询 `GET /partials/services`；行内按钮 `hx-post /action?label=..&op=..` 返回单行 partial 替换自身；详情按钮懒加载 `GET /partials/detail`。
- 列表来源：**只显示 `~/Library/LaunchAgents` 下有 plist 文件的 agent**（plist 视角，有意排除 `launchctl list` 里的 com.apple.* 噪音）。
- 单个服务状态：先 `launchctl print gui/$UID/<label>` 拿运行时信息；失败则回退 `launchctl print-disabled` 判断 enabled。
- 模板渲染注意：仅含 `{{define}}` 块的模板文件要用 define 名执行（如 `"services"`），不是文件名。
- 静态资源：embed 根带 `static/` 目录前缀，必须用 `fs.Sub(web.Static, "static")` 再挂 FileServer。

## launchctl 本机已知行为（重要）

1. 本机 macOS 的 `launchctl print` **不支持 `-json`**，输出为旧式文本块（`key = value`，嵌套 `{...}`）。解析逻辑在 `parseBlock`，顶层块的起始行是 `<target> = {`，解析需从下一行开始（`idx := i + 1`）。
2. `print` 输出中程序参数块叫 `arguments`（不是 program arguments），成员是不带引号的裸行。
3. 被 disable / 未 bootstrap 的服务 `launchctl print` 报 "Could not find service ... in domain"，属正常回退路径，不是错误。
4. `launchctl print-disabled gui/$UID` 输出 `"label" => disabled|enabled` 行格式。

## 安全模型

- 仅监听 127.0.0.1；bcrypt 密码 + HMAC session cookie（7 天），SameSite=Lax 下不额外做 CSRF token。
- 密码哈希与 secret key 明文存于 `~/.localctl/config.json`（0600），勿在代码或日志中输出其内容。

## 约定

- 用户域固定 `gui/$UID`，不支持 system 域（管理 LaunchDaemons 需要 sudo，超出本项目范围）。
- 管理操作直接生效于 launchctl；Stop 用 `kill SIGTERM`，Disable 是持久化的（重启后仍禁用）。
- 危险操作（Stop/Disable/Unload）在模板里用 `hx-confirm` 二次确认，新增操作请保持此约定。
- 代码注释与 UI 文案混用中英文；模板中的用户可见文案为中文。
