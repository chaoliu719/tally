## Why

claude.ai 连接器每隔最多 1 小时(access token TTL)就要重新走一遍 `/authorize`,
而登录页要求人工粘贴 tally 静态 token —— 于是用户每小时被打断一次去输 token。
OAuth 2.1 为这个场景准备的机制是 refresh token,但当前实现刻意没做(见
`token.go` D6 注释、project 边界「无 refresh token」)。加上它,用户只在首次接入
输一次 token,此后由客户端静默续期。

## What Changes

- `/token` 兑换 `authorization_code` 时,除 access token 外**同时签发一个
  refresh token**:沿用现有无状态 HMAC 签名值(新增 `typ:"rt"`),绑定
  audience(本 server 规范 URI),有效期 90 天。
- `/token` **新增受理 `grant_type=refresh_token`**:校验签名有效、类型为 `rt`、
  audience 为本 server、未过期;通过则签发新的 access token,并**滚动轮换**——
  一并签发一个新的、过期时间顺延 90 天的 refresh token 返回。任一校验失败返回
  HTTP 400 OAuth 错误响应。
- 授权服务器元数据 `grant_types_supported` 增加 `refresh_token`。
- access token TTL 从 1 小时提升到 24 小时(spec 上限)。
- refresh token 与 access token、授权码一样**无服务端状态、无单个撤销**;轮换
  `TALLY_OAUTH_SIGNING_SECRET` 一次性作废全部凭据(含所有 refresh token)。
- 更新 project 边界描述:把「无 refresh token」改为「有滚动轮换的无状态
  refresh token」,与已有的「无账户、无 scope、无 per-token 撤销」并列。

不改变:静态 token 仍是 `/authorize` 唯一登录门禁;Claude Code / Desktop 直接用
静态 bearer token 的路径不受影响;单用户、无账户系统不变。

## Capabilities

### New Capabilities
<!-- 无 -->

### Modified Capabilities
- `mcp-oauth-authorization`: 授权服务器元数据声明 `refresh_token` grant;令牌
  端点在授权码兑换时附带签发 refresh token,并新增受理 `refresh_token` 授权
  类型(校验 + 滚动轮换)。access token 实际 TTL 调为 24 小时,仍在既有
  「不超过 24 小时」表述内,不改该要求。

`mcp-transport` 不受影响:它只约束受保护资源元数据,不涉及
`grant_types_supported`。

## Impact

- 代码:`mcp/internal/oauth/token.go`(新增 refresh token 载荷/签发/校验、
  调整 `accessTokenTTL`)、`mcp/internal/oauth/server.go`(`handleToken` 分支、
  `tokenResponse` 增加 `refresh_token` 字段、元数据 `grant_types_supported`)。
- 测试:`token_test.go` / `server_test.go` 补 refresh 流程用例。
- 部署:纯代码变更,镜像重建即可;无 schema 变更、无新环境变量、无数据迁移。
  已签发的旧 access token 在其 TTL 内继续有效。
- 文档:`openspec/project.md` 的传输与认证段落。
