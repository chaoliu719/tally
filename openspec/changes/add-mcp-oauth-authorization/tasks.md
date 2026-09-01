## 1. 首次连接验证(spike 已折叠进真实实现,见 design D7)

- [x] 1.1 真实实现部署到 `tally.liuchao.life` 后,server 端对 `/authorize` `/token`
  `/register` `/.well-known/*` 开详细请求日志(方法、路径、query、关键 body 字段;
  不记 token 明文)
- [ ] 1.2 **待用户操作** — 在真实 claude.ai「添加自定义连接器」指向
  `https://tally.liuchao.life/mcp`,走完 OAuth 授权。读服务端日志
  (`ssh aliyun 'cd ~/tally-mcp && docker compose logs -f | grep oauth'`)核对:是否调
  `/register`、`redirect_uri` 确切值、`resource` 参数、`code_challenge_method`、access
  token 过期后是否重跑授权。与 D5/D6 一致则继续;不一致则更新 design.md 并按需改代码。
  headless 部分已验证:register→authorize(token)→token(PKCE)→用 access token 调
  `list_ledgers` 返回「猫兔2026」全通(见 7.1)

## 2. 部署:切子域名

- [x] 2.1 aliyun Caddy 加 `tally.liuchao.life` site block,反代 `localhost:16355`(根路径,
  不剥前缀),与现有 `mcp.liuchao.life/tally` 并存;`curl https://tally.liuchao.life/healthz` → 200
- [x] 2.2 `~/tally-mcp/.env` 加 `TALLY_OAUTH_SIGNING_SECRET`(`openssl rand -hex 32`);
  `docker compose up -d` 后进程正常启动

## 3. 配置与启动

- [x] 3.1 `mcp/internal/bootstrap/config.go`:新增 `TALLY_OAUTH_SIGNING_SECRET`,未设置则
  `LoadConfig` 返回错误;`config_test.go` 覆盖「缺失即启动失败」和「三密钥独立读取」
- [x] 3.2 定义 server 规范 resource URI 的来源(配置项或从请求推导),确保 audience 校验
  和 metadata 里的 `resource` 用同一个值;单测覆盖

## 4. 令牌封装

- [x] 4.1 实现授权码的签发/校验:自包含签名值,绑定 `code_challenge`/`redirect_uri`/`resource`/
  `exp`(≤10min);单测覆盖签名篡改、过期、绑定不一致各返回失败
- [x] 4.2 实现 access token 的签发/校验:绑定 `aud`/`iat`/`exp`/`jti`;单测覆盖
  audience 不符、过期、旧密钥签发一并失效
- [x] 4.3 实现 `client_id` 的签发/校验:编码 `redirect_uris` 列表;单测覆盖据此校验
  `redirect_uri` 精确匹配、非法 redirect URI 被拒

## 5. 资源服务器侧(mcp-transport 变更)

- [x] 5.1 `mcp/internal/authn`:中间件改为双通道 —— 先常量时间比对静态 token,不中则按
  OAuth access token 校验(4.2);两条路径都失败返回 401。用 go-sdk `auth.RequireBearerToken`
  承接 `WWW-Authenticate` 生成
- [x] 5.2 挂载 `GET /.well-known/oauth-protected-resource`(go-sdk
  `auth.ProtectedResourceMetadataHandler`),`resource` = 规范 URI,`authorization_servers`
  指向本机;该端点免认证
- [x] 5.3 `authn_test.go` / 传输层测试:静态 token 仍放行、有效 OAuth token 放行、
  错误/过期/audience 不符的 token 返回 401 且响应头带 `WWW-Authenticate` 指向 resource metadata、
  well-known 端点免认证返回正确 JSON —— 对应 mcp-transport spec 的全部 Scenario

## 6. 授权服务器侧(mcp-oauth-authorization 新能力)

- [x] 6.1 `GET /.well-known/oauth-authorization-server`:RFC 8414 元数据,声明四个端点 +
  `code` response type + `authorization_code` grant + `S256`;免认证;单测校验字段
- [x] 6.2 `GET /authorize`:校验 `client_id`/`redirect_uri`(精确匹配 4.3)/`response_type`/
  `code_challenge`/`code_challenge_method=S256`/`resource`;校验失败返回 400;通过则渲染
  单字段登录 HTML。单测覆盖缺 PKCE、redirect_uri 不匹配
- [x] 6.3 `POST /authorize`:常量时间比对静态 token;匹配 → 签发授权码(4.1)+ 302 到
  `redirect_uri?code=...&state=...`;不匹配 → 重渲染登录页不签码不重定向。单测覆盖两条路径
- [x] 6.4 `POST /token`:`grant_type=authorization_code`,校验授权码签名/过期/`redirect_uri`/
  `resource` 一致 + `SHA256(code_verifier)==code_challenge`;失败返回 400 OAuth 错误;
  成功返回 `{access_token, token_type:"Bearer", expires_in}`。单测覆盖 PKCE 失败、码过期、码篡改
- [x] 6.5 `POST /register`:校验每个 redirect_uri 是 `https`/`http://localhost` 且无 fragment,
  返回 201 + `client_id`(4.3),无 `client_secret`。单测覆盖合法注册、非法 redirect URI
- [x] 6.6 `mcp/cmd/tally-mcp/main.go` 的 `buildMux` 挂载 6.1–6.5 全部路由;确认与 `/mcp`
  的 StreamableHTTP handler 路径不冲突

## 7. 端到端与收尾

- [x] 7.1 写一个 curl/脚本,在本地对 `buildMux` 走完整流程:发现 → register → authorize
  (带 token)→ token(带 PKCE)→ 用返回的 access token 调 `list_ledgers` 成功;
  静态 token 直连 `list_ledgers` 仍成功(回归)
- [ ] 7.2 **待用户操作** — 已部署到 aliyun(`tally-mcp:oauth-dev` 镜像 + Caddy
  `tally.liuchao.life` + `.env` 加 `TALLY_OAUTH_SIGNING_SECRET`/`TALLY_PUBLIC_BASE_URL`);
  在真实 claude.ai 添加连接器指向 `https://tally.liuchao.life/mcp`,完成 OAuth 授权,在
  网页版对话里 `list_ledgers` 返回「猫兔2026」
- [x] 7.3 更新 `~/.claude/settings.json` 的 `TALLY_MCP_URL`、`plugin/.mcp.json` 注释/文档、
  `mcp/README.md`、`plugin/README.md` 的地址为 `tally.liuchao.life`;`.env.example` 加
  `TALLY_OAUTH_SIGNING_SECRET`
- [x] 7.4 更新 `openspec/config.yaml` 的 `context`:transport/认证段改为「静态 bearer 或
  MCP OAuth(2.1 + PKCE + DCR)」,标注取代原「不做 JWT/OAuth」决定,并写明边界(单用户、
  无账户、无 scope、无 per-token 撤销)
- [ ] 7.5 **收尾(观察后)** — 确认 claude.ai + Claude Code 都稳定走 `tally.liuchao.life`
  后:移除 Caddy 的 `mcp.liuchao.life/tally` 路由;把 aliyun compose 的镜像从
  `tally-mcp:oauth-dev` 换成正式发布的 `ghcr.io/chaoliu719/tally-mcp` tag(需要先发版);
  仓库 `mcp/docker-compose.yml` 已改好(含新 env 变量)
- [x] 7.6 `openspec validate "add-mcp-oauth-authorization" --strict` 通过;`go test ./...` 通过
