## MODIFIED Requirements

### Requirement: 提供授权服务器元数据
Server SHALL 在 `GET /.well-known/oauth-authorization-server` 提供符合 RFC 8414 的授权
服务器元数据,至少声明 `issuer`、`authorization_endpoint`、`token_endpoint`、
`registration_endpoint`、支持的 `response_types`(`code`)、`grant_types`
(`authorization_code`、`refresh_token`)、`code_challenge_methods_supported`(`S256`)。
所有端点 URI SHALL 使用 HTTPS。

#### Scenario: 客户端发现授权端点
- **WHEN** 客户端 `GET /.well-known/oauth-authorization-server`
- **THEN** server 返回 JSON 元数据,其中的 `authorization_endpoint` / `token_endpoint` / `registration_endpoint` 都是本 server 上的 HTTPS URL

#### Scenario: 该端点无需认证
- **WHEN** 客户端在没有凭据的情况下请求该元数据端点
- **THEN** server 正常返回,不返回 401

#### Scenario: 元数据声明 refresh_token 授权类型
- **WHEN** 客户端读取元数据的 `grant_types_supported`
- **THEN** 该数组同时包含 `authorization_code` 和 `refresh_token`

### Requirement: 令牌端点校验授权码与 PKCE 并签发 access token
`POST /token` SHALL 接受 `grant_type=authorization_code`,校验:授权码签名有效、未过期、
其绑定的 `redirect_uri` 与本次请求一致、其绑定的 `resource` 与本次请求一致、
`code_verifier` 经 SHA-256 后与授权码绑定的 `code_challenge` 相等。任一校验失败 SHALL
返回 OAuth 错误响应(HTTP 400),不签发 token。全部通过则返回
`{ "access_token", "token_type": "Bearer", "expires_in", "refresh_token" }`,其中
`refresh_token` 是一个绑定本 server 规范 URI、可用于后续 `grant_type=refresh_token`
请求的凭据。

#### Scenario: 有效的授权码兑换
- **WHEN** `POST /token` 携带一个未过期的授权码和正确的 `code_verifier`,`redirect_uri` 与 `resource` 与授权时一致
- **THEN** server 返回 200,响应体包含 `access_token`(`token_type` 为 `Bearer`)和一个 `refresh_token`

#### Scenario: PKCE 校验失败
- **WHEN** `code_verifier` 经 SHA-256 后与授权码绑定的 `code_challenge` 不相等
- **THEN** server 返回 400 OAuth 错误,不签发 token,也不签发 refresh token

#### Scenario: 授权码已过期或签名被篡改
- **WHEN** `POST /token` 携带的授权码已超过其过期时间,或签名校验不通过
- **THEN** server 返回 400 OAuth 错误,不签发 token

## ADDED Requirements

### Requirement: 令牌端点受理 refresh_token 授权并滚动轮换
`POST /token` SHALL 接受 `grant_type=refresh_token`,以 `refresh_token` 参数携带凭据。
server SHALL 依次校验:refresh token 签名有效、类型为 refresh token、audience 为本
server 规范 URI、未过期。任一校验失败 SHALL 返回 OAuth 错误响应(HTTP 400),不签发
任何新凭据。全部通过则 SHALL 返回一个新的 access token,并**同时**返回一个新的
refresh token,其过期时间自本次签发起顺延(滚动轮换),响应体形如
`{ "access_token", "token_type": "Bearer", "expires_in", "refresh_token" }`。

refresh token SHALL 是自包含的签名值,携带 audience(本 server 规范 URI)、签发时间、
一个自签发起不超过 90 天的过期时间,并用与 access token 相同的 OAuth 签名密钥
(`TALLY_OAUTH_SIGNING_SECRET`)签名。server SHALL NOT 为已签发的 refresh token 保存
任何服务端状态,也 SHALL NOT 提供单个 refresh token 的撤销机制;轮换签名密钥即一并
作废全部 refresh token。

#### Scenario: 用有效 refresh token 换新 access token
- **WHEN** `POST /token` 携带 `grant_type=refresh_token` 和一个签名有效、audience 为本 server、未过期的 refresh token
- **THEN** server 返回 200,响应体包含一个新的 `access_token` 和一个新的 `refresh_token`

#### Scenario: refresh token 已过期
- **WHEN** `POST /token` 携带的 refresh token 已超过其 90 天过期时间
- **THEN** server 返回 400 OAuth 错误,不签发新的 access token 或 refresh token

#### Scenario: refresh token 签名无效或 audience 不符
- **WHEN** `POST /token` 携带的 refresh token 签名校验失败,或其 audience 不是本 server 规范 URI
- **THEN** server 返回 400 OAuth 错误,不签发任何凭据

#### Scenario: 把 access token 当作 refresh token 提交
- **WHEN** `POST /token` 以 `grant_type=refresh_token` 提交的值实际上是一个 access token(类型不符)
- **THEN** server 返回 400 OAuth 错误,不签发任何凭据

#### Scenario: 签名密钥轮换后的旧 refresh token
- **WHEN** `TALLY_OAUTH_SIGNING_SECRET` 被更换,之后收到一个用旧密钥签发的 refresh token
- **THEN** 签名校验失败,该 refresh token 及所有旧 refresh token 一并失效
