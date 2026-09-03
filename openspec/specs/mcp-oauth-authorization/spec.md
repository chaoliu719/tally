# mcp-oauth-authorization Specification

## Purpose
定义 tally-mcp 内建的最小 OAuth 2.1 授权服务器:它如何向 MCP 客户端声明自己、如何用
现有的静态 token 作为唯一登录门禁完成授权码 + PKCE 流程、如何签发与校验无状态签名
access token,以及如何做无状态的动态客户端注册。目标是让 tally-mcp 作为 claude.ai
自定义连接器接入,而不引入账户系统、scope、per-token 撤销或任何外部认证依赖。

## Requirements

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

### Requirement: 授权码流程以静态 token 为登录门禁
`GET /authorize` SHALL 校验 `client_id`、`redirect_uri`、`response_type=code`、
`code_challenge`、`code_challenge_method=S256`、`resource` 参数,校验通过后向用户呈现
一个单字段登录界面,要求输入 tally 的静态 token。提交(`POST /authorize`)时,server
SHALL 以常量时间比对该 token;匹配则签发一个授权码并 302 重定向到
`redirect_uri?code=<授权码>&state=<原样回传>`;不匹配则重新呈现登录界面并提示失败,不
签发授权码。授权码 SHALL 是自包含签名值,绑定 `code_challenge`、`redirect_uri`、
`resource`、一个不超过 10 分钟的过期时间,server 不为其保存任何状态。

#### Scenario: 参数合法且 token 正确
- **WHEN** `/authorize` 参数全部合法,用户在登录界面输入了与服务端配置一致的静态 token
- **THEN** server 302 重定向到 `redirect_uri`,查询参数带上签名授权码和原样的 `state`

#### Scenario: token 错误
- **WHEN** 用户在登录界面输入的 token 与服务端配置不一致
- **THEN** server 不签发授权码,不重定向到 `redirect_uri`,而是重新显示登录界面

#### Scenario: 缺少 PKCE 参数
- **WHEN** `/authorize` 请求缺少 `code_challenge` 或 `code_challenge_method` 不是 `S256`
- **THEN** server 返回 400,不呈现登录界面

#### Scenario: redirect_uri 与注册值不一致
- **WHEN** `/authorize` 的 `redirect_uri` 与该 `client_id` 注册时的任何一个 redirect URI 都不精确匹配
- **THEN** server 返回 400,不重定向到该 `redirect_uri`

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

### Requirement: 签发的 access token 是无状态签名令牌
access token SHALL 是自包含的签名值,携带 audience(本 server 规范 URI)、签发时间、
不超过 24 小时的过期时间、一个唯一标识符,并用专用签名密钥签名。server 校验 access
token 时 SHALL 依次检查:签名有效、audience 为本 server、未过期。server SHALL NOT 为
已签发的 access token 保存任何服务端状态,也 SHALL NOT 提供单个 token 的撤销机制。

#### Scenario: 校验一个自己签发的 token
- **WHEN** server 收到一个自己此前签发、未过期、audience 正确的 access token
- **THEN** 校验通过,请求被处理

#### Scenario: 过期的 token
- **WHEN** access token 的过期时间已过
- **THEN** 校验失败,server 返回 401

#### Scenario: 签名密钥轮换后的旧 token
- **WHEN** `TALLY_OAUTH_SIGNING_SECRET` 被更换,之后收到一个用旧密钥签发的 access token
- **THEN** 签名校验失败,该 token 及所有旧 token 一并失效

### Requirement: 动态客户端注册是无状态的
`POST /register` SHALL 接受符合 RFC 7591 的注册请求,校验其中每个 `redirect_uri` 都是
`https` 或 `http://localhost` 且不含 fragment,然后返回一个 `client_id`。该 `client_id`
SHALL 是把这批 `redirect_uris` 编码进去的自包含签名值,使 `/authorize` 能在不保存注册
记录的情况下校验后续请求的 `redirect_uri`。server SHALL NOT 为注册结果保存任何服务端
状态。返回的客户端 SHALL 是公开客户端(不签发 `client_secret`)。

#### Scenario: 合法注册请求
- **WHEN** `POST /register` 提交一个或多个合法的 `https` redirect URI
- **THEN** server 返回 201 和一个 `client_id`,不返回 `client_secret`

#### Scenario: 用注册返回的 client_id 发起授权
- **WHEN** 客户端拿 `/register` 返回的 `client_id` 请求 `/authorize`,且 `redirect_uri` 是注册时提交过的那批之一
- **THEN** `/authorize` 的 `redirect_uri` 校验通过(server 未查询任何存储)

#### Scenario: 非法 redirect URI
- **WHEN** `POST /register` 提交的某个 `redirect_uri` 不是 `https`/`http://localhost`,或含 fragment
- **THEN** server 返回 400,不签发 `client_id`

### Requirement: 未配置签名密钥则拒绝启动
Server 启动时,用于签发/校验 OAuth 授权码与 access token 的签名密钥
(`TALLY_OAUTH_SIGNING_SECRET`)SHALL 通过启动配置提供,且与 MCP 传输层的静态 token、
以及确认令牌的密钥相互独立。该密钥未配置时,server SHALL 启动失败并报错。

#### Scenario: 缺少 OAuth 签名密钥
- **WHEN** server 启动时 `TALLY_OAUTH_SIGNING_SECRET` 未设置
- **THEN** server 启动失败并输出说明该配置缺失的错误,不以缺少该配置的状态提供服务

#### Scenario: 三个密钥相互独立
- **WHEN** server 签发 access token
- **THEN** 使用 `TALLY_OAUTH_SIGNING_SECRET`,而非 MCP 静态 token 或确认令牌密钥

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
