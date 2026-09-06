## Context

见 proposal.md - Why。补充当前实现事实:

- `mcp/internal/mcpserver/mcpserver.go:14` 现在是 `mcp.NewServer(&mcp.Implementation{Name, Version}, nil)`,
  两个调用方:`mcp/cmd/tally-mcp/main.go:49`(`serverName`/`serverVersion` 常量)与
  `mcp/internal/tools/testutil_test.go:60`。
- go-sdk v1.7.0 的 `mcp.Implementation` 已有 `Title` / `WebsiteURL` / `Icons []Icon` 字段;
  `mcp.Icon{Source(json "src"), MIMEType, Sizes, Theme}`。SDK 把它们原样放进 `initialize` 结果。
- HTTP 路由在 `mcp/cmd/tally-mcp/main.go:buildMux`(48-86)。`authn.Middleware` 只包 `/mcp`
  (main.go:64);`/healthz`、`oauthSrv.Routes` 注册的 `/.well-known/*` 都是免认证的裸 handler。
- 资源内嵌的先例:`mcp/internal/widgets/widgets.go:19,30-34` 用 `import _ "embed"` +
  `//go:embed` 到 `string`。本 change 要 embed 二进制,目标类型是 `[]byte`。
- Dockerfile 只 `COPY cmd ./cmd` 和 `COPY internal ./internal`,新资源放 `internal/` 下即被覆盖。
- 部署:aliyun,Caddy `tally.liuchao.life { reverse_proxy localhost:16355 }` 整站透传;
  `cfg.PublicBaseURL`(`bootstrap.Config.PublicBaseURL`,已 `TrimRight` 去尾斜杠)= `https://tally.liuchao.life`。
- 源图 `Tally.png`(仓库根,1254×1254 RGBA,3D 风格小票+硬币)。本机有 Pillow 12.3.0。

## Goals / Non-Goals

**Goals:**
- serverInfo 带上 `title` / `websiteUrl` / `icons`,客户端一旦支持即自动显示。
- 图标经一个公开、可被任意 HTTP 客户端直接抓取的 URL 提供(而非 `data:` URI 或 MCP 资源)。
- 图标观感是「圆角、非直角」的透明背景 PNG,不是贴在白色方块上的图。
- 资源准备可复现:换图重跑一个脚本即可。

**Non-Goals:**
- 不做 light/dark 双版本(`Icon.Theme`)—— 先上单版透明图,深浅底都可读;将来需要再加。
- 不改 `openspec/config.yaml` 的 context 块 —— serving 一张品牌图标与已有的 `/.well-known/*`、
  `/favicon.ico` 同类,属传输层元数据,不是 widget 那种「破例托管 UI」的哲学例外。
- 不动 Caddyfile / docker-compose.yml / Dockerfile。
- 不引入图片处理的 Go 依赖 —— 处理在构建前一次性完成,产物直接进仓库。

## Decisions

### D1: 图标走静态 HTTP 路由,不用 `data:` URI,也不用 MCP 资源

`icons[].src` = `cfg.PublicBaseURL + "/icon.png"`。server 在 `buildMux` 注册 `/icon.png` 与
`/favicon.ico` 两条免认证路由,handler 查 `internal/assets` 内嵌字节并写出。

- **否决 `data:` URI 内联进 serverInfo**:会让每次 `initialize` 响应体多背几十 KB 的 base64;
  远程 server 的托管 URL 没有额外往返成本(客户端本来就要发 HTTP)。
- **否决做成 `ui://` MCP 资源**:图标要被浏览器/连接器 UI 用普通 HTTP GET 抓,不是被 MCP 客户端
  через `resources/read` 读;MCP 资源那套(`widgets` 包、profile mime、TTL)不适用。
- 免认证是对的:`authn.Middleware` 只包 `/mcp`,新路由与 `/healthz` 一样公开;spec 里显式要求
  「图标端点的免认证不放宽 `/mcp` 的认证要求」以防回归。

### D2: `mcpserver.New` 改为接收整个 `*mcp.Implementation`

```go
func New(impl *mcp.Implementation) *mcp.Server { return mcp.NewServer(impl, nil) }
```

`mcpserver` 包定位是「transport/协议 wiring」,不该硬编码 "Tally" 这类产品文案。品牌字段的值
在 `main.go` 构造:

```go
server := mcpserver.New(&mcp.Implementation{
    Name: serverName, Version: serverVersion,
    Title:      "Tally",
    WebsiteURL: "https://github.com/chaoliu719/tally",
    Icons: []mcp.Icon{{Source: cfg.PublicBaseURL + "/icon.png", MIMEType: "image/png", Sizes: []string{"512x512"}}},
})
```

- **`websiteUrl` 选 GitHub 仓库而非 `https://tally.liuchao.life`**:后者根路径只反代 MCP 端点,
  浏览器打开是协议错误页,不是给人看的落地页;GitHub 地址与 `plugin/.claude-plugin/plugin.json`
  的 `repository` 一致。
- **否决给 `New` 加可变参数 / options**:只有两个调用方,直接传 struct 最简单;测试辅助传
  `&mcp.Implementation{Name: "tally-mcp-test", Version: "0.0.0"}`。

### D3: `internal/assets` 包 + 构建前资产生成脚本

- `assets.go`:`import _ "embed"`;`//go:embed icon.png favicon.ico` 到 `[]byte`;导出
  `Bytes(name string) (data []byte, contentType string, ok bool)`。`favicon.ico` 的
  `Content-Type` 用 `image/x-icon`。
- `generate.py`(随包提交):Pillow 从 `../../../Tally.png` →
  1. 载入,转 RGBA;把接近纯白/浅灰的背景像素改为透明(阈值容差),得到只剩插画的透明图;
  2. 按 alpha 边界裁剪,再按 ~8% 边距重排到正方形画布;
  3. 缩放到 512×512(`LANCZOS`);
  4. 套圆角矩形遮罩,圆角半径 ≈ 0.22 × 512 —— 避免「纯方直角」观感,不加任何方形底板;
  5. 写 `icon.png`;并导出 `favicon.ico`(16/32/48 三尺寸)。
- 产物 `icon.png` / `favicon.ico` 提交进仓库。脚本是可复现凭据,不在构建流水线里跑。

- **否决 SVG**:源图是 3D 光栅渲染,矢量化会丢质感;PNG 512 足够,客户端自行缩放。
- **否决在 Go 运行时处理图片**:要引 `golang.org/x/image` 之类依赖,且每次启动做无谓工作;
  产物固定,进仓库最省。

## Risks / Trade-offs

- **[claude.ai 端尚未读取 serverInfo `icons`]** → 本 change 不依赖它:字段填了不会有坏处,
  `/favicon.ico` 立即在浏览器标签页等场景生效;客户端支持铺开后图标自动出现。
- **[`Tally.png` 背景可能已是透明 / 也可能是浅色底]** → `generate.py` 同时处理两种情况
  (已透明则去背一步近乎 no-op);产物肉眼验收。
- **[圆角半径 / 去背阈值需要肉眼调]** → 参数集中在脚本顶部常量;验收标准写进 tasks。
- **[二进制体积增加]** → 一张 512 PNG + 一个多尺寸 ico,合计约几十 KB,可忽略。
- **回滚**:字段与路由都是纯新增,回退镜像即可,无数据迁移;旧客户端本就忽略未知 serverInfo 字段。

## Migration Plan

1. 生成资产 → 改代码 → `cd mcp && go build ./... && go test ./...` 全绿。
2. 构建镜像推 GHCR `:latest` → `ssh aliyun` `cd ~/tally-mcp && docker compose pull && docker compose up -d`
   → `curl https://tally.liuchao.life/healthz`(见 memory: tally-mcp deploy flow)。
3. 验证:`curl -sI https://tally.liuchao.life/icon.png` 200 + `image/png`;走一次 `initialize`
   看 serverInfo 三字段;claude.ai 里重新添加 tally 连接器观察图标 / favicon。
4. 回滚:回退到上一镜像 tag,`docker compose up -d`。
