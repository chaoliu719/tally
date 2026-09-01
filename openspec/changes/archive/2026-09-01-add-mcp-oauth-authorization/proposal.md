## Why

claude.ai 网页版的自定义连接器**只支持 OAuth**——没有自定义请求头字段,纯静态 bearer
token 的 server 在网页版连不上(Claude Code / Desktop 不受影响)。要让 tally-mcp 在
网页版可用,server 必须按 MCP 授权规范(OAuth 2.1 + PKCE + Resource Indicators + 动态
客户端注册)对外提供 OAuth。

这与 `openspec/config.yaml` 里「认证用单个静态 bearer token,不做 JWT/OAuth」的既定
决定冲突。但看清楚:MCP 的这套 OAuth **正是** config 同一段所说的「未来多用户会采用的
『Agent 时代』的认证方式」,不是它拒绝的「传统 web 认证」(注册 / 邮箱验证 / 密码重置 /
2FA / 多会话管理——本 change 一个都不加)。单用户场景下增量很小,原因:没有用户模型要
建;access token 复用现有 `confirmation_token` 的无状态签名令牌模式;go-sdk(v1.7.0,
已是依赖)已提供资源服务器那半边的中间件与元数据处理器。

## What Changes

- **部署:路径前缀 → 子域名。** 从 `mcp.liuchao.life/tally/*`(Caddy 剥前缀)切到
  `tally.liuchao.life`,让 RFC 9728 的 `/.well-known/*` 发现路径回到根路径,避免子路径
  下 well-known 路径插入的兼容性问题。tally-mcp 的规范 resource URI 变为
  `https://tally.liuchao.life/mcp`。
- **资源服务器 (RS) 侧**(`mcp-transport` 能力变更):
  - `/mcp` 的认证中间件改为**双通道**:接受现有静态 bearer token,**或**一个由本 server
    的 AS 签发、audience 绑定本机的 OAuth access token。
  - 未授权时返回 401 并带 `WWW-Authenticate: Bearer resource_metadata="..."` 头。
  - 新增 `GET /.well-known/oauth-protected-resource`(RFC 9728),声明授权服务器位置。
- **授权服务器 (AS) 侧**(新增 `mcp-oauth-authorization` 能力,同进程):
  - `GET /.well-known/oauth-authorization-server`(RFC 8414)。
  - `GET/POST /authorize`:OAuth 2.1 授权码流程 + PKCE。用户面对一个**单字段登录表单**,
    输入现有静态 token 作为门禁;匹配则 302 回 `redirect_uri?code=...`。
  - `POST /token`:校验授权码 + PKCE `code_verifier`,签发 access token。
  - `POST /register`:动态客户端注册(RFC 7591),无状态——允许的 `redirect_uri` 编入
    签名的 `client_id`,不落库。
  - access token:HMAC 签名、自包含 `{aud, iat, exp, jti}`、无 token 表;有效期约 1 小时。
  - 新增启动配置 `TALLY_OAUTH_SIGNING_SECRET`(与 MCP token、confirmation secret 相互独立)。
- **config.yaml**:更新 transport/认证段,标注本 change 取代「不做 JWT/OAuth」那句,并
  界定新边界(单用户、Agent 时代认证、无账户、无 scope、无 per-token 撤销)。

**BREAKING**:对外访问地址从 `mcp.liuchao.life/tally/mcp` 变为 `tally.liuchao.life/mcp`。
已配置的客户端(Claude Code settings.json、Desktop 连接器)需更新 URL。静态 bearer token
本身不变,裸 bearer 认证方式保留。

## 非目标

- 不加账户系统 / 用户表 / uid;不加注册、邮箱验证、密码重置、2FA、会话管理。
- 不加 OAuth scopes——access token 要么全权、要么无效。
- 不加 per-token 撤销——无状态令牌只能靠轮换 `TALLY_OAUTH_SIGNING_SECRET` 全部作废。
- v1 不签发 refresh token——access token 过期后由客户端重跑一次授权流程。
- AS 不作为通用授权服务器对外,只服务 tally 自身这一个资源。
- 不引入外部认证服务(Auth0 / WorkOS / Cloudflare Access 等)或任何 OAuth 库以外的依赖。

## Capabilities

### New Capabilities
- `mcp-oauth-authorization`: tally-mcp 内建的最小 OAuth 2.1 授权服务器——授权服务器
  元数据、授权码 + PKCE 流程(以静态 token 为登录门禁)、令牌签发、无状态动态客户端
  注册,以及无状态签名 access token 的格式与校验规则。

### Modified Capabilities
- `mcp-transport`: 「每个请求必须携带有效的静态 bearer token」放宽为「静态 bearer token
  或本 server 签发的 OAuth access token」;新增未授权响应必须带 `WWW-Authenticate` 指向
  受保护资源元数据、以及必须提供 `/.well-known/oauth-protected-resource` 的要求。

## Impact

- `mcp/internal/authn/`:中间件改为双通道,新增 OAuth access token 校验(HMAC 签名 +
  过期 + audience)。
- `mcp/internal/`:新增 OAuth AS 包(authorize / token / register 处理器、授权码与
  access token 的签名封装)。复用 `write-confirmation` 已有的 HMAC 签名模式。
- `mcp/cmd/tally-mcp/main.go`:`buildMux` 挂载新的 well-known 与 OAuth 路由。
- `mcp/internal/bootstrap/config.go`:新增 `TALLY_OAUTH_SIGNING_SECRET`。
- 依赖:使用 `github.com/modelcontextprotocol/go-sdk/auth` 与 `.../oauthex`(已在 go.mod);
  不新增第三方依赖。
- 部署:aliyun Caddy 从 `mcp.liuchao.life` 的 `handle_path /tally/*` 改为独立
  `tally.liuchao.life` site block;`~/tally-mcp/.env` 增加新密钥。
- 文档:`mcp/README.md`、`plugin/README.md`、`plugin/.mcp.json` 的默认 URL,以及本机
  `~/.claude/settings.json` 的 `TALLY_MCP_URL`。
- `openspec/config.yaml` 的 `context` 段。
- 无数据库 schema 变更;无记账工具行为变更。
