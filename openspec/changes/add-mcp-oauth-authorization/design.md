## Context

见 proposal.md - Why。塑造本设计的约束:

- MCP 授权规范(2025-06-18)把 server 拆成**资源服务器 (RS)** 和**授权服务器 (AS)** 两个
  角色,可以同进程。RS 侧:RFC 9728 受保护资源元数据 + `WWW-Authenticate` 401 + token
  audience 校验。AS 侧:OAuth 2.1 + PKCE + RFC 8414 元数据,DCR 是 SHOULD。
- `github.com/modelcontextprotocol/go-sdk` v1.7.0(已在 go.mod)的 `auth` 包提供
  `RequireBearerToken(verifier, opts)` 中间件和 `ProtectedResourceMetadataHandler(...)`,
  `oauthex` 包提供元数据 / DCR / audience 的类型。**SDK 覆盖 RS 那半边**;AS 的流程
  (authorize / token / register)SDK 只提供客户端实现,服务端要自己写。
- 代码库已有无状态 HMAC 签名令牌的先例:`write-confirmation` 的 `confirmation_token`
  (自包含、独立密钥、带过期、无服务端状态)。OAuth 的授权码和 access token 复用同一
  模式。
- 当前部署在 `mcp.liuchao.life/tally/*`(Caddy `handle_path` 剥前缀)。

## Goals / Non-Goals

**Goals:**
- tally-mcp 能作为 claude.ai 网页版自定义连接器接入,走标准 MCP OAuth 发现流程。
- 不新增任何第三方依赖、不新增服务端持久状态。
- 静态 bearer token 路径完整保留(Claude Code / Desktop 不受影响)。

**Non-Goals:**
- 见 proposal.md「非目标」。特别是:无账户、无 scope、无 per-token 撤销、v1 无 refresh
  token、AS 不做通用授权服务器。
- 不改任何记账工具、不改数据库 schema。

## Decisions

### D1: AS 与 RS 同进程,`tally.liuchao.life` 子域名根路径挂载

- 所有 OAuth / well-known 路由挂在 tally-mcp 进程的 `http.ServeMux` 上,和 `/mcp`、
  `/healthz` 并列。
- **部署切子域名**:`mcp.liuchao.life/tally/*` → `tally.liuchao.life`。原因:RFC 9728 /
  RFC 8414 的 `/.well-known/*` 发现在子路径下要做路径插入
  (`/.well-known/oauth-protected-resource/tally/mcp`),Claude 的发现实现能否正确处理
  是未知数;根路径没有这个问题。子域名在 Caddy 上只多一个 site block。
- 规范 resource URI = `https://tally.liuchao.life/mcp`(无尾斜杠)。access token 的
  audience、`/authorize` 与 `/token` 的 `resource` 参数都必须等于它。
- 备选:保留路径前缀,赌 Claude 的路径插入实现正确 —— 被否,风险不对称,切子域名成本极低。

### D2: 认证中间件双通道,静态 token 优先

`/mcp` 的中间件按顺序尝试:

```
Authorization: Bearer <cred>
   │
   ├─ constant-time 等于静态 TALLY_MCP_TOKEN?  ──▶ 放行(现有路径)
   │
   └─ 否 → 当作 OAuth access token:HMAC 验签 → 检查 aud==规范URI → 检查未过期
              │
              ├─ 全过 ──▶ 放行
              └─ 任一失败 ──▶ 401 + WWW-Authenticate: Bearer
                              resource_metadata="https://tally.liuchao.life/.well-known/oauth-protected-resource"
```

- 用 SDK 的 `auth.RequireBearerToken`,把「静态 token 或签名 token」的判断塞进
  `TokenVerifier`。`opts` 里配置 resource metadata URL,让 SDK 生成 `WWW-Authenticate`。
- 备选:两个中间件串联 —— 没必要,一个 verifier 里分支更简单。

### D3: 授权码和 access token 都是 HMAC 签名的自包含值

复用 `write-confirmation` 的封装思路(具体函数在实现阶段决定是否抽公共包):

| | 绑定内容 | 过期 | 用途 |
|---|---|---|---|
| 授权码 | `code_challenge`、`redirect_uri`、`resource` | ≤10 分钟 | `/authorize` 签发,`/token` 校验一次 |
| access token | `aud`(规范 URI)、`iat`、`exp`、`jti` | ≤24 小时(初定 1h) | `/token` 签发,`/mcp` 中间件校验 |
| client_id | 注册的 `redirect_uris` 列表 | 不过期 | `/register` 签发,`/authorize` 校验 redirect_uri |

- 全部用 `TALLY_OAUTH_SIGNING_SECRET` 签名,与 `TALLY_MCP_TOKEN`、
  `TALLY_CONFIRMATION_SECRET` 独立(doctrine:密钥各自独立)。
- 编码格式(JWT vs 自定义 base64url)实现阶段定;倾向跟 `confirmation_token` 保持一致。
- **无 token 表、无授权码一次性消费记录**。授权码 10 分钟内可被重放 —— 缓解:短过期 +
  PKCE(没有 `code_verifier` 重放无意义)。可接受。

### D4: `/authorize` 的登录门禁 = 现有静态 token

单用户,没有账户。`/authorize` 呈现一个单字段 HTML 表单(「粘贴 tally token」),
`POST` 时 constant-time 比对 `TALLY_MCP_TOKEN`。匹配即视为「资源所有者已同意」。

- 复用 `authn` 已有的常量时间比对。
- 表单是进程内嵌的最小 HTML,无 CSS 框架、无 JS。
- 备选:独立密码 / 委托 Google 登录 —— 都是多余的新概念或新依赖。

### D5: DCR 无状态,开放注册

`POST /register` 校验 redirect_uri 合法性后,返回 `client_id = sign(redirect_uris)`。
不落库、不签发 `client_secret`(公开客户端)。

- 开放注册不是风险:`/authorize` 的 token 门禁才是守卫,拿到 `client_id` 也没用。
- 让 Claude 网页版零配置接入(不用手填 Advanced settings 的 Client ID)。
- 备选:要求预注册 + 用户手填 —— UX 差,且需要一个配置界面。

### D6: v1 不签发 refresh token

`/token` 只返回 access token。过期后 Claude 会在 `/mcp` 拿到 401 → 重跑发现 + 授权
流程(用户重新粘一次 token)。

- 规范对**公开客户端**要求 refresh token 必须轮换 —— 不签发就不触发这条要求。
- 代价:每 ~1h(或选更长的 24h)重登一次。先接受,烦了再加 refresh(届时需要轮换
  追踪,可能就得引入一点状态)。
- 这条是 spike 的重点之一:确认 Claude 能优雅处理「无 refresh token + access token 过期」。

## D7: spike 折叠进真实实现

原计划先写 throwaway stub 探 claude.ai 行为。改为:直接按规范 + Claude MCP OAuth 的
已知行为(DCR + PKCE `S256` + `resource` 参数,这套在 MCP 生态已成熟)写真实实现,
部署时开**详细请求日志**,第一次在真实 claude.ai 加连接器即验证。有偏差就改真实代码,
不再走一版 stub。D5(无状态开放 DCR)、D6(不发 refresh token)按「假设成立、首次连接
验证」处理;首次连接若发现 Claude 强制 refresh token 或 DCR 行为不符,回到 design 修订
D5/D6 并可能改 `mcp-oauth-authorization` spec。

## Risks / Trade-offs

- **[Claude 网页版实际 OAuth 行为无文档]** 走不走 DCR、redirect_uri 具体值、能否接受
  AS 与 RS 同源、强不强制 refresh token、对 `resource` 参数的处理 → 按 D7,真实实现带
  详细日志,首次连接即验证;偏差回改真实代码与 design。
- **[授权码可重放 10 分钟]** 无一次性消费记录 → PKCE 兜底(无 `code_verifier` 不可用)+
  短过期。若 spike 发现 Claude 行为异常,再考虑加一个内存 `jti` 去重集(仍无持久状态)。
- **[`/authorize` 是公网上的密码表单]** → 常量时间比对(已有)+ 可加简单速率限制。token
  泄露风险与现状(静态 token 直接就是凭据)持平,不更差。
- **[子域名迁移是 BREAKING]** 已配置的客户端要改 URL → 迁移计划里列清楚三处(settings.json、
  Desktop 连接器、plugin 文档),静态 token 不变降低影响。
- **[doctrine 反转]** `config.yaml` 明确「不做 OAuth」→ 本 change 显式取代该句,proposal
  和 config context 同步更新;非目标清单锁死范围防止 OAuth 把项目带向多用户 / scope。
- **[well-known 路由与 `/mcp` 的 SDK handler 冲突]** StreamableHTTPHandler 挂在 `/mcp`,
  well-known 挂在 `/.well-known/*` → ServeMux 路径不重叠,但要确认 SDK handler 不假设
  自己在根路径。实现阶段验证。

## Migration Plan

1. 实现并本地跑通(静态 token 路径回归 + 新 OAuth 流程用 curl 脚本走一遍)。
2. aliyun:`.env` 加 `TALLY_OAUTH_SIGNING_SECRET`;Caddy 加 `tally.liuchao.life` site
   block(先与 `mcp.liuchao.life/tally` 并存)。
3. 用真实 claude.ai 添加连接器指向 `https://tally.liuchao.life/mcp`,完成 OAuth 接入验证。
4. 更新本机 `~/.claude/settings.json`、plugin 文档、README 的 URL。
5. 观察一段时间后,移除 Caddy 的 `mcp.liuchao.life/tally` 路由。
6. 回滚:OAuth 路由是纯增量,静态 token 路径不变;回滚只需还原 Caddy 路由和客户端 URL,
   OAuth 代码留着不影响裸 bearer 使用。

## Open Questions

- access token 有效期取 1h 还是 24h —— 取决于 spike 里 Claude 无 refresh token 时的重登
  体验;不影响 spec 和任务拆分,实现时定。
- 授权码 / token 的编码用不用严格 JWT —— 影响 `oauthex` 能否直接复用其校验;design 阶段
  spike 后定,不改 spec。
