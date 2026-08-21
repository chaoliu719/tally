## 1. 项目初始化

- [ ] 1.1 用 `go mod init tally` 建立 module,验证 `go.mod` 存在且声明的 Go 版本不低于 ezbookkeeping 当前要求
- [ ] 1.2 添加 `github.com/mayswind/ezbookkeeping` 依赖并锁定到具体 tag(无 tag 则用 commit pseudo-version),验证对其 `pkg/services`/`pkg/models` 的空引用能 `go build` 通过
- [ ] 1.3 查阅官方 MCP Go SDK(`github.com/modelcontextprotocol/go-sdk`)当前文档,确认工具注册/HTTP 传输的具体 API,添加依赖并锁定版本,验证 `go build` 通过
- [ ] 1.4 按 design.md 的目录结构建好 `cmd/tally-mcp`、`internal/bootstrap`、`internal/authn`、`internal/mcpserver`、`internal/tools`,各放一个占位文件,验证 `go build ./...` 全仓库通过

## 2. 配置与 bootstrap

- [ ] 2.1 实现配置加载:`TALLY_MCP_TOKEN`(必需)、`TALLY_DEFAULT_CURRENCY`(可选,默认 CNY)、DB 文件路径,验证缺少 `TALLY_MCP_TOKEN` 时进程以非零退出码退出并打印清晰错误信息
- [ ] 2.2 复用 `pkg/datastore` 初始化 SQLite 并执行迁移,验证首次启动后生成的 DB 文件包含 ezbookkeeping 的完整 schema(用 `sqlite3 <file> .tables` 之类的命令核对)
- [ ] 2.3 实现"不存在则创建"的单用户 bootstrap(含随机生成、永不校验的密码字段),验证:清空 DB 后连续启动两次,数据库中的 user 表始终只有一行

## 3. 认证中间件

- [ ] 3.1 实现 bearer token 校验中间件(用 `crypto/subtle.ConstantTimeCompare` 而非普通字符串比较),验证:不带 token 或 token 错误的请求返回认证错误且未触达任何工具逻辑;携带正确 token 的请求正常放行

## 4. MCP 协议层与传输

- [ ] 4.1 用官方 SDK 搭建 `mcp.Server`,实现可扩展的工具注册表,把其 HTTP handler 挂在 `net/http` 的 `/mcp` 路径,另加 `/healthz` 健康检查
- [ ] 4.2 验证 `initialize` → `tools/list` → `tools/call` → `ping` 全流程可以用一个真实 MCP client 或手工构造的 JSON-RPC 请求跑通(此时工具表可以还是空的)
- [ ] 4.3 验证调用未注册的工具名返回结构化 JSON-RPC 错误而不是进程崩溃;验证请求体不是合法 JSON 时同样返回结构化错误且进程继续存活,可处理下一个请求

## 5. 账户管理工具

- [ ] 5.1 实现 `list_accounts`(调用 `pkg/services.AccountService` 的查询方法),验证空账本调用返回空列表而不是错误
- [ ] 5.2 实现 `manage_account` 的创建路径,验证 `account-management/spec.md` 中三个场景全部可复现:合法参数创建成功且出现在 `list_accounts` 结果中、缺必填字段被拒绝、不支持的币种被拒绝

## 6. 分类管理工具

- [ ] 6.1 实现 `list_categories`(调用 `pkg/services.TransactionCategoryService` 的查询方法),验证空账本调用返回空列表
- [ ] 6.2 实现 `manage_category` 的创建路径,验证 `category-management/spec.md` 中两个场景全部可复现:合法参数创建成功且出现在 `list_categories` 结果中、缺必填字段被拒绝

## 7. 交易记录工具

- [ ] 7.1 实现 `create_transaction`(收入/支出类型,调用 `pkg/services.TransactionService.CreateTransaction`),验证 `transaction-recording/spec.md` 的"提供有效信息记录交易"场景:交易创建后所属账户余额按类型正确增减
- [ ] 7.2 验证同一 spec 中"引用不存在的账户或分类"与"缺少必填字段"两个失败场景:请求被拒绝、不写入交易、账户余额不变
- [ ] 7.3 实现 `get_transaction`,验证已存在交易能查到完整信息、不存在的 ID 返回明确的"未找到"错误而非空结果或崩溃
- [ ] 7.4 实现 `search_transactions`,支持按时间范围/账户/分类筛选,验证 spec 中三个场景:无筛选条件返回全部、按时间范围筛选生效、筛选结果为空时返回空列表而非错误

## 8. 端到端验证

- [ ] 8.1 从全新的空 SQLite 文件开始,只通过 MCP 工具调用(不直接操作数据库)完整走一遍:启动 server → `manage_account` 建账户 → `manage_category` 建分类 → `create_transaction` 记一笔支出 → `search_transactions` 查到这笔交易 → `get_transaction` 按其 ID 查到同一笔交易且账户余额已扣减,验证 proposal.md 中描述的最小闭环成立

## 9. 文档

- [ ] 9.1 写 README:环境变量说明、构建与启动命令、如何在 Claude Code/Claude Desktop 中把这个 server 配置为一个 HTTP + bearer token 的远程 MCP server,验证按文档步骤能从零跑起来并连上一个真实 MCP 客户端
