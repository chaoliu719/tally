## ADDED Requirements

### Requirement: Server 在握手时声明品牌标识

`initialize` 响应的 serverInfo 中,除 `name` 与 `version` 外,server SHALL 同时提供:

- `title` —— 面向人的展示名;
- `websiteUrl` —— 一个 `https://` 开头的绝对 URL,指向该项目的说明页;
- `icons` —— 一个非空数组,每个条目 SHALL 携带 `src`(一个 `https://` 开头的绝对 URL,指向可被
  无凭据访问的图片资源)、`mimeType`、`sizes`。

这些字段是纯展示信息,不影响协议协商;客户端不识别时应忽略。

#### Scenario: 握手完成后 serverInfo 携带品牌字段

- **WHEN** 客户端完成 `initialize` 握手
- **THEN** 响应的 serverInfo 中 `title` 非空、`websiteUrl` 是 `https://` 开头的绝对 URL、`icons`
  数组非空,且每个 icon 的 `src` 是 `https://` 开头的绝对 URL

#### Scenario: 旧客户端忽略未知字段仍能完成握手

- **WHEN** 一个不识别 `title` / `websiteUrl` / `icons` 的客户端发送 `initialize`
- **THEN** 握手正常完成,协议版本与能力协商不受这些字段影响

### Requirement: 图标资源在免认证的固定路径提供

server SHALL 在固定路径 `GET /icon.png` 提供 serverInfo `icons[].src` 所指向的图片,并在
`GET /favicon.ico` 提供站点图标。这两个端点 SHALL 不要求任何访问凭据(与 `/healthz`、
`/.well-known/*` 一致),响应 SHALL 携带与内容匹配的 `Content-Type`。

#### Scenario: 无凭据获取 serverInfo 指向的图标

- **WHEN** 客户端在没有 `Authorization` 头的情况下 `GET` serverInfo 里 `icons[].src` 指向的路径
- **THEN** server 返回 200 与图片字节,`Content-Type` 为对应的图片类型,不返回 401

#### Scenario: 无凭据获取站点 favicon

- **WHEN** 客户端在没有 `Authorization` 头的情况下 `GET /favicon.ico`
- **THEN** server 返回 200 与图标字节,不返回 401

#### Scenario: 图标端点不影响受保护端点

- **WHEN** 客户端向 `/mcp` 发送不带凭据的请求
- **THEN** server 仍返回 401(图标端点的免认证不放宽 `/mcp` 的认证要求)
