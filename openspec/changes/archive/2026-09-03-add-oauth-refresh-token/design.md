## Context

见 proposal.md — Why。现有 OAuth 实现(`mcp/internal/oauth/`)是全无状态的:授权码、
access token、client_id 都是 `base64url(JSON) + "." + base64url(HMAC-SHA256)` 两段式
签名值,统一由 `TALLY_OAUTH_SIGNING_SECRET` 签名,靠 `typ` 字段区分。没有 token 存储,
没有 per-token 撤销,轮换密钥即全体失效。`accessTokenTTL = time.Hour`,`/token` 只有
`authorization_code` 一个分支,`tokenResponse` 没有 `refresh_token` 字段,元数据
`GrantTypesSupported` 只有 `["authorization_code"]`。

project.md 当前把「无 refresh token」列为 OAuth 边界之一,本 change 取代该条(类比
archive/add-mcp-oauth-authorization 取代「不做 JWT/OAuth」)。

## Goals / Non-Goals

**Goals:**
- 首次接入后,claude.ai 能在 90 天内静默续期,用户不再被反复要求粘贴静态 token。
- 保持现有无状态签名模型:refresh token 复用同一密钥、同一两段式编码、同一 `typ`
  区分机制,不引入任何存储。

**Non-Goals:**
- 单个 refresh token 撤销、token 家族追踪、重放检测(replay detection)——与无状态
  模型冲突,继续只靠密钥轮换作废全体。
- 改动 `/authorize` 登录门禁:静态 token 仍是唯一门禁。
- 改动 Claude Code / Desktop 的静态 bearer token 路径。

## Decisions

### D1: refresh token 沿用 `encode`/`decode`,新增 `typ:"rt"`
新增 `refreshTokenPayload{ Typ, Audience, IssuedAt, ExpiresAt, JTI }`,与
`accessTokenPayload` 同构,仅 `typ` 值不同(`typRefreshToken = "rt"`)。新增
`issueRefreshToken(secret, audience, jti)` 和 `verifyRefreshToken(secret, token,
wantAudience, now)`,与 access token 的一对函数完全对称。
- 备选:JWT。否决——现有代码刻意不用 JWT(HMAC 两段式已够,`token.go` 注释明确),
  没有理由为 refresh token 单独引入。
- 备选:给 refresh token 单独一个签名密钥。否决——spec 已规定三密钥独立
  (transport 静态 token / confirm / OAuth),refresh token 属 OAuth 授权服务器
  职责,与 access token 同源同命是正确的语义,也让「轮换 OAuth 密钥作废全体」
  这条保持简单。

### D2: 滚动轮换(rotating refresh token)
每次 `grant_type=refresh_token` 兑换都签发一个新 refresh token,`ExpiresAt` 自当次
起 +90 天。因为无状态,旧 refresh token 在它自己的 90 天到期前仍可用(无法主动
作废)——这是可接受的:窗口有界,且每次成功续期都把有效链条向后推 90 天,正常使用
下用户永不需要重输。
- 备选:不轮换,首个 refresh token 固定 90 天。更简单,但用户每 90 天必须重新
  授权一次;轮换后「只要 90 天内用过一次就不用管」体验明显更好。
- 备选:refresh token 永不过期。否决——用户已选 90 天滚动方案;有界窗口在无
  per-token 撤销的前提下是必要的安全兜底。

### D3: access token TTL 1h → 24h
`accessTokenTTL = 24 * time.Hour`。spec 表述本就是「不超过 24 小时」,无需改要求。
减少续期往返频率;即便 refresh 出问题,也有 24h 缓冲而非 1h。

### D4: `handleToken` 按 `grant_type` 分派
`handleToken` 顶部读 `grant_type`,`authorization_code` 走现有逻辑(末尾追加签发
refresh token),`refresh_token` 走新分支,其他返回 `unsupported_grant_type`。
`refresh_token` 分支:取 `refresh_token` 参数 → `verifyRefreshToken` → 失败返回
`invalid_grant`(HTTP 400)→ 成功则 `issueAccessToken` + `issueRefreshToken`,
`Cache-Control: no-store`,返回 `tokenResponse`。

### D5: `tokenResponse` 增加 `RefreshToken string json:"refresh_token,omitempty"`
两个分支都填。`omitempty` 只是防御性写法;两条路径都会带上。

### D6: 元数据 `GrantTypesSupported = ["authorization_code", "refresh_token"]`
`handleAuthServerMetadata` 里改一行。RFC 8414 客户端据此决定是否尝试 refresh。

## Risks / Trade-offs

- [旧 refresh token 在 90 天窗口内无法作废] → 接受。无状态模型的固有取舍,已在
  spec 与 project 边界写明;需要即时失效时轮换 `TALLY_OAUTH_SIGNING_SECRET`
  (作废全体,所有客户端重新授权一次)。
- [refresh token 泄露 = 90 天内可持续换 access token] → 缓解:凭据只在 TLS 下
  传输(所有端点强制 HTTPS);audience 绑定,泄露的 refresh token 不能用于其他
  资源;轮换密钥是最终手段。风险等级与「静态 token 泄露」相当,而静态 token
  本就是唯一门禁。
- [claude.ai 是否发送 `resource` 参数到 refresh 请求] → refresh 分支不要求
  `resource` 参数(RFC 8707 对 refresh 是可选);audience 已固化在 refresh token
  内。若客户端发了且与本 server 不符,可选择性拒绝——按现有 `/token` 对
  `resource` 的处理风格(非空才校验)。

## Migration Plan

1. 合并代码,重建镜像 `ghcr.io/chaoliu719/tally-mcp:latest`。
2. `ssh aliyun` → `~/tally-mcp` → `docker compose pull && docker compose up -d`。
3. 无 schema 变更、无新环境变量、无数据迁移。已签发的旧 access token 在其原
   1h TTL 内继续有效;claude.ai 下次续期时走新的 refresh 流程。
4. 回滚:`docker compose` 切回上一个镜像 tag 即可;新签发的 refresh token 在旧
   代码下会因 `grant_type` 不受理而失效,claude.ai 回落到重新授权(即当前现状),
   无数据损坏。

## Open Questions

无。
