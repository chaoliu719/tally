# mcp-transport Specification

## Purpose
定义 MCP server 对外暴露的协议与传输契约:客户端如何连接、认证、发现工具、调用工具,以及请求不合法时如何响应。不包含任何具体记账工具的业务规则。

## Requirements

### Requirement: MCP 端点通过 JSON-RPC 提供协议方法
Server SHALL 在一个 HTTP 端点上,通过 JSON-RPC 2.0 提供 `initialize`、`ping`、`tools/list`、`tools/call` 四个方法。

#### Scenario: 客户端完成握手
- **WHEN** 客户端发送 `initialize` 请求
- **THEN** server 返回其支持的 MCP 协议版本与能力声明

#### Scenario: 客户端发现可用工具
- **WHEN** 客户端发送 `tools/list` 请求
- **THEN** server 返回当前已注册的全部工具及其输入/输出 JSON Schema

#### Scenario: 客户端调用已注册工具
- **WHEN** 客户端发送 `tools/call`,`name` 为一个已注册的工具名,`arguments` 满足该工具的输入 schema
- **THEN** server 将请求分发给对应工具的处理逻辑,并把处理结果作为 JSON-RPC 响应返回

#### Scenario: 客户端调用未注册的工具名
- **WHEN** 客户端发送 `tools/call`,`name` 不在已注册工具列表中
- **THEN** server 返回 JSON-RPC 错误响应,不执行任何记账操作,也不导致进程崩溃

#### Scenario: 客户端探活
- **WHEN** 客户端发送 `ping` 请求
- **THEN** server 立即返回成功响应,不依赖数据库查询结果

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

### Requirement: 已注册工具集合在发现与调用间保持一致
`tools/list` 返回的工具集合 SHALL 与 `tools/call` 实际能够成功分发的工具集合完全一致。

#### Scenario: 新工具注册后立即可发现可调用
- **WHEN** 一个新工具被注册进工具表
- **THEN** 该工具同时出现在后续的 `tools/list` 响应中,且能被后续的 `tools/call` 成功分发

### Requirement: 非法请求不会导致进程崩溃
Server SHALL 对格式错误的 JSON-RPC 请求(如非法 JSON、缺少必填字段)返回结构化的错误响应,而不是让进程崩溃或无响应。

#### Scenario: 请求体不是合法 JSON
- **WHEN** 客户端发送的请求体无法解析为合法 JSON
- **THEN** server 返回错误响应,进程保持运行,可以继续处理后续请求
