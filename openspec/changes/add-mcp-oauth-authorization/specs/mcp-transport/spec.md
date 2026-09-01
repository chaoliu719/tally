## RENAMED Requirements

- FROM: `### Requirement: 每个请求必须携带有效的静态 bearer token`
- TO: `### Requirement: 每个请求必须携带有效的访问凭据`

## MODIFIED Requirements

### Requirement: 每个请求必须携带有效的访问凭据
除非请求携带一个有效的访问凭据,否则 server SHALL 拒绝该请求,且不执行请求中指定的任何 MCP 方法(包括 `tools/call` 触发的记账操作)。有效凭据是以下之一:与服务端配置一致的静态 bearer token;或一个由本 server 的授权服务器签发、audience 绑定本 server 规范 URI、且未过期的 OAuth access token(见 `mcp-oauth-authorization`)。两种凭据都通过 `Authorization: Bearer <凭据>` 头传递。

#### Scenario: 携带正确 token
- **WHEN** 请求的 `Authorization` 头携带与服务端配置一致的静态 bearer token
- **THEN** server 正常处理该请求

#### Scenario: 携带有效的 OAuth access token
- **WHEN** 请求的 `Authorization` 头携带一个签名有效、audience 为本 server 规范 URI、且未过期的 OAuth access token
- **THEN** server 正常处理该请求

#### Scenario: 缺失或错误的 token
- **WHEN** 请求未携带 `Authorization` 头,或携带的凭据既不是正确的静态 token、也不是有效的 OAuth access token
- **THEN** server 返回 401,不执行请求中的 MCP 方法,也不在错误信息中泄露正确静态 token 或签名密钥的任何片段

#### Scenario: access token 的 audience 不是本 server
- **WHEN** 请求携带的 OAuth access token 签名有效但 audience 指向另一个资源
- **THEN** server 拒绝该请求并返回 401,不接受也不转发该 token

## ADDED Requirements

### Requirement: 未授权响应支持 OAuth 授权服务器发现
返回 401 时,server SHALL 附带 `WWW-Authenticate: Bearer` 响应头,其中包含指向受保护资源元数据文档的 `resource_metadata` 参数。server SHALL 在 `GET /.well-known/oauth-protected-resource` 提供该文档(符合 RFC 9728),文档中 `authorization_servers` 字段 SHALL 至少包含一个授权服务器,`resource` 字段 SHALL 为本 server 的规范 URI。

#### Scenario: 无凭据请求收到可发现的 401
- **WHEN** 客户端向 `/mcp` 发送不带 `Authorization` 头的请求
- **THEN** server 返回 401,响应头 `WWW-Authenticate` 携带 `resource_metadata` 指向 `/.well-known/oauth-protected-resource`

#### Scenario: 客户端拉取受保护资源元数据
- **WHEN** 客户端 `GET /.well-known/oauth-protected-resource`
- **THEN** server 返回 JSON,包含本 server 的规范 `resource` URI 和至少一个 `authorization_servers` 条目

#### Scenario: 该发现端点无需认证
- **WHEN** 客户端在没有任何凭据的情况下请求 `/.well-known/oauth-protected-resource`
- **THEN** server 正常返回元数据文档,不返回 401
