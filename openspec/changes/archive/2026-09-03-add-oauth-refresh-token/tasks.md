## 1. token.go — refresh token 原语

- [x] 1.1 在 `mcp/internal/oauth/token.go` 新增 `typRefreshToken = "rt"` 常量与
      `refreshTokenTTL = 90 * 24 * time.Hour`;把 `accessTokenTTL` 改为
      `24 * time.Hour`。验证:`go build ./...` 通过。
- [x] 1.2 新增 `refreshTokenPayload`(字段与 `accessTokenPayload` 同构)、
      `issueRefreshToken(secret, audience, jti string) (token string, expiresIn int)`、
      `verifyRefreshToken(secret, token, wantAudience string, now time.Time) (time.Time, error)`,
      与 access token 的一对函数对称,`typ` 校验为 `typRefreshToken`。
      验证:`token_test.go` 新增 `TestRefreshTokenRoundTrip` / `TestRefreshTokenRejections`
      ——签发后 verify 通过;篡改签名 / 错 audience / 过期 / 传入 access token 各返回 error。

## 2. server.go — /token 分派与响应

- [x] 2.1 `tokenResponse` 增加 `RefreshToken string json:"refresh_token,omitempty"`。
      在现有 `authorization_code` 分支成功路径,签发 refresh token 并填入响应。
      验证:`server_test.go` 的 `fullFlow` 断言响应含非空 `refresh_token`。
- [x] 2.2 重构 `handleToken`:顶部按 `grant_type` 分派为 `handleAuthCodeGrant` /
      `handleRefreshTokenGrant`。`authorization_code` 保持现逻辑;`refresh_token`
      分支——读 `refresh_token` 参数 → `verifyRefreshToken` 失败返回
      `invalid_grant`(HTTP 400)→ 成功则经 `writeTokenPair` 签发新 access + 新
      refresh token,`Cache-Control: no-store`;未知 `grant_type` 返回
      `unsupported_grant_type`。
      验证:`server_test.go` 新增 `TestRefreshTokenGrantRotates` /
      `TestRefreshTokenGrantRejections` / `TestTokenRejectsUnknownGrantType`。
- [x] 2.3 `handleAuthServerMetadata` 的 `GrantTypesSupported` 改为
      `["authorization_code", "refresh_token"]`。
      验证:`TestAuthServerMetadata` 断言数组含 `refresh_token`。

## 3. 收尾

- [x] 3.1 `openspec validate add-oauth-refresh-token --strict` 通过;
      `go test ./...` 全绿;`gofmt`/`go vet ./...` 无输出。
- [x] 3.2 更新 `openspec/config.yaml` 的 project context 传输与认证段落:把
      「无 refresh token」改为「滚动轮换的无状态 refresh token(90 天,复用 OAuth
      签名密钥,无 per-token 撤销)」。
- [x] 3.3 部署步骤(纯代码变更):合并后重建镜像
      `ghcr.io/chaoliu719/tally-mcp:latest` → `ssh aliyun` →
      `cd ~/tally-mcp && docker compose pull && docker compose up -d`。
      无 schema 变更、无新环境变量、无数据迁移。
