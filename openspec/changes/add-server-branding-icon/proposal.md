## Why

tally-mcp 作为 claude.ai 自定义连接器接入时,连接器 UI 只能显示一个通用地球图标 —— 因为 `initialize`
响应的 serverInfo 里只有 `name` + `version`,没有任何品牌信息。MCP spec(SEP-973)已定义 serverInfo
的 `title` / `websiteUrl` / `icons`,go-sdk v1.7.0 也已支持这些字段。现在补齐是零风险前置动作:客户端
支持一旦铺开就自动生效,在那之前 `/favicon.ico` 已能在浏览器标签页等场景露出。

## What Changes

- `initialize` 响应的 serverInfo 新增三个品牌字段:`title`(展示名 "Tally")、`websiteUrl`(指向
  GitHub 仓库)、`icons`(至少一个条目,`src` 为可公开访问的绝对 URL,带 `mimeType` 与 `sizes`)。
- server 在两个**免认证**的固定路径提供图标字节:`GET /icon.png`(serverInfo `icons[].src` 指向它)、
  `GET /favicon.ico`。与现有的 `/healthz`、`/.well-known/*` 一样不需要 `Authorization`。
- 图标资源随二进制内嵌发布;源图为仓库已有的 `Tally.png`,构建期一次性处理为透明背景、非直角
  (圆角)的 512×512 PNG 与多分辨率 ico。
- 无破坏性变更:纯新增字段与新增路由,旧客户端忽略未知的 serverInfo 字段。

## Capabilities

### New Capabilities
（无)

### Modified Capabilities
- `mcp-transport`: 新增一条 Requirement —— server 必须在 serverInfo 中声明品牌标识(`title` /
  `websiteUrl` / 非空 `icons`),且必须在免认证的固定路径提供图标资源与 `/favicon.ico`。现有的
  握手 / 认证 / 工具发现 Requirement 不变。

## Impact

- **代码**
  - `mcp/internal/mcpserver/mcpserver.go`:`New` 改为接收整个 `*mcp.Implementation`,保持该包通用。
  - `mcp/cmd/tally-mcp/main.go`:构造带 `Title` / `WebsiteURL` / `Icons` 的 Implementation;在
    `buildMux` 加 `/icon.png`、`/favicon.ico` 两条免认证路由。
  - `mcp/internal/assets/`(新包):`//go:embed` 图标字节 + `generate.py` 资产生成脚本 + 生成产物
    `icon.png` / `favicon.ico`。
  - `mcp/internal/tools/testutil_test.go`、`mcp/cmd/tally-mcp/main_test.go`:跟随 `New` 签名;新增
    路由与 serverInfo 断言。
- **配置 / 部署**:无需改 Caddyfile(aliyun 上整站透传到 :16355)、docker-compose.yml、Dockerfile
  (`COPY internal ./internal` 已覆盖新资源)。
- **依赖**:不新增。仅用到已在用的 `github.com/modelcontextprotocol/go-sdk` v1.7.0 已有的字段。
- **spec**:`openspec/specs/mcp-transport/spec.md` 随本 change 归档时同步。
