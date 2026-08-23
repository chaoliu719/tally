## Context

动机见 [proposal.md](proposal.md)。这里写清楚确认令牌怎么签、`revision` 具体 hash 哪些字段、分类环检测怎么做,以及 schema 怎么变。

现状:`internal/tools/{accounts,categories,transactions}.go` 只有创建路径;`internal/store/schema.sql`/`queries.sql` 是 sqlc 手写 SQL + 生成代码,没有独立迁移框架,只有一个 schema 版本;`internal/authn` 是现有 bearer token 认证中间件的先例(常量时间比较、专门的中间件包)。

## Goals / Non-Goals

**Goals:**
- `manage_account`/`manage_category` 支持 `operation=update/delete`,复用同一个工具、同一个输入 struct
- 一个可被两者复用、以后也能被批量操作复用的无状态确认令牌机制
- `create_transaction` 支持 `balance_adjustment`
- 分类模型砍掉 `type` 字段与两层限制,`parent_id` 支持任意深度嵌套,并对"挪动"做环检测

**Non-Goals:**
- id 的线上编码格式(十进制字符串 → JSON number)——拆到单独 change
- 批量操作的 preview/apply——这次只搭好确认令牌这一层通用机制,批量操作本身不在范围内
- 账户/分类的隐藏、排序
- 转账类型交易、汇率
- 给分类重新引入任何形式的类型/层级约束——这次是明确移除,不是换一种方式保留

## Decisions

### 确认令牌:自包含的两段式字符串,不引入 JWT 库

`internal/confirm`(新包,与 `internal/authn` 平级)提供 `Issue`/`Verify` 两个函数。令牌格式是 `base64url(JSON payload) + "." + base64url(HMAC-SHA256(payload, secret))`——两段,没有 JWT 的 header/alg 协商(算法固定死,不需要协商,也没必要为了这一个用途引入完整 JWT 依赖)。payload 是:

```go
type payload struct {
    Action    string `json:"action"`     // "delete_account" / "delete_category"
    ID        string `json:"id"`         // 目标资源 id(与工具线上格式一致的十进制字符串)
    Revision  string `json:"revision"`   // hex(sha256(...)),见下面"revision 覆盖哪些字段"
    ExpiresAt int64  `json:"expires_at"` // unix 秒,签发时间 + 15 分钟
}
```

`Verify(secret, token, wantAction, wantID, currentRevision, now) error` 依次检查:签名(`hmac.Equal`,常量时间)、`Action == wantAction`、`ExpiresAt > now`、`Revision == currentRevision`。四项任意一项不通过就返回描述性错误(调用方把它转成工具的错误返回,不区分"哪一项失败"对外暴露过多细节——见 write-confirmation spec 的"拒绝并提示需要重新预览"这类统一措辞)。

**为什么不用服务端存储 token 状态(比如"已使用"标记)**:tally 是单进程、单用户、SQLite 本地文件,引入一张 `pending_confirmations` 表纯粹是为了防重放,而重放已经被"apply 前重新活查一次前置条件"这道天然存在的检查挡住了(第二次用同一个 token 删除同一个账户时,账户已经不在了,活查会直接失败)。为防一个已经被结构性防住的问题再加一张表、多一次写入,不划算。

### revision 覆盖哪些字段

`revision = hex(sha256(json.Marshal(fields)))`,`fields` 按资源类型分别是:

- 账户删除:`{Name, Type, Comment, Currency, TxCount}`——`TxCount` 是 `SELECT COUNT(*) FROM transactions WHERE account_id = ?` 的结果,决定了删除是否被允许;其余四个字段变化本身不影响能不能删,但如果它们变了,说明 preview 时展示给用户看的"要删的是哪个账户"已经不是当前状态,同样应该让确认失效。
- 分类删除:`{Name, ParentID, ChildCount, TxCount}`——`ChildCount` 是 `SELECT COUNT(*) FROM categories WHERE parent_id = ?`,`TxCount` 是 `SELECT COUNT(*) FROM transactions WHERE category_id = ?`。

两种情况用 JSON 编码字段再 hash,不用分隔符拼字符串——避免字段值本身包含分隔符字符导致的边界歧义,实现也更直接(`encoding/json` 已经是项目依赖)。

### apply 时的活查兜底,与 revision 校验是两道独立防线

`Verify` 通过之后,真正执行 DELETE 之前,在同一个 `sql.Tx` 里再查一次 `TxCount`(账户)或 `ChildCount`+`TxCount`(分类),不为零就拒绝、回滚。这道检查和 revision 校验的关系:revision 校验是"提前量",能在更早的阶段用更友好的报错("状态已变化,请重新 preview")拦住漂移;活查是唯一真正堵住 TOCTOU 竞态窗口的手段(revision 只能感知它 hash 进去的那些字段,活查感知的是执行前一刻的真实状态)。两者都做,原因见 [write-confirmation/spec.md](specs/write-confirmation/spec.md) 的"全部校验通过后仍在执行前重新确认一次"这条场景。

### `TALLY_CONFIRMATION_SECRET`:启动时必需,独立于 `TALLY_MCP_TOKEN`

`internal/bootstrap/config.go` 新增一个环境变量,校验方式与现有 `TALLY_MCP_TOKEN` 完全对称(`LoadConfig` 里未设置就返回 fatal 错误)。不复用 `TALLY_MCP_TOKEN` 的原因:两者签名保护的东西性质不同(一个是"这个 HTTP 请求是不是合法客户端发的",一个是"这个删除操作是不是刚刚被同一次 preview 授权过")——混用一个密钥,意味着任何能读到 bearer token 的人也能自己伪造确认令牌,而 bearer token 出现在每一次请求的 HTTP 头里,暴露面比确认令牌大得多。分开之后,泄露 bearer token 不等于能绕过确认这道关卡。

### 分类环检测:递归查子孙集合

`internal/store/queries.sql` 新增一条用 `WITH RECURSIVE` 写的查询,给定一个分类 id,返回它的全部子孙 id(不含自身)。`operation=update` 挪动分类时,校验新的 `parent_id`:不等于自身 id,且不在这个子孙集合里。这是唯一能正确处理任意深度嵌套下"挪到自己子孙下面会成环"的办法——链式地一层层查父分类反过来验证在深层嵌套下要么要递归查,要么要缓存全量父子关系,不如直接一条递归 CTE 查子孙集合直观。

### Schema:`categories` 表删除 `type` 列

```sql
CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    parent_id   INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
```

不再有 `CHECK` 约束限制层级(原本也没有——两层限制之前完全是 Go 代码里校验的,不是数据库约束,这次连 Go 校验一起去掉)。目前没有真实账本数据(和 `replace-ezbookkeeping-backend` 那次一样),不需要写 `ALTER TABLE ... DROP COLUMN`;直接改 `schema.sql` 里的 `CREATE TABLE`,本地如果有旧结构的 `tally.db` 直接删掉重建。

### `manage_account`/`manage_category` 的 `operation` 字段:必需,不设默认值

`operation` 不设"缺省等于 create"这种兼容行为——现在还没有真实调用方需要向后兼容,显式好过隐式,和这次其他地方"拒绝不合法输入而不是静默兜底"的一贯做法一致(`currency`/`balance` 传给 update 被拒绝、`category_id` 传给 `balance_adjustment` 被拒绝,都是同一个原则)。

### 输出 `status` 字段

`ManageAccountOutput`/`ManageCategoryOutput` 新增 `Status string`,取值 `created`/`updated`/`pending_confirmation`/`deleted`,四种都伴随把资源当前(或最后已知)信息放在既有的 `Account`/`Category` 字段里——`pending_confirmation` 时是"即将被删除的资源当前状态",`deleted` 时是"刚刚被删除的资源的最后状态",保持"输出里总有资源信息"这一点在四种状态下一致,调用方不需要按 status 分支才能拿到基本信息。

## Risks / Trade-offs

- **[风险] 递归 CTE 查子孙集合,分类树很深/很宽时会变慢。** → 单用户个人记账的分类数量级(几十到几百条)远远谈不上"深/宽到有影响";这次不做进一步优化。
- **[风险] `revision` 用 JSON 编码后 hash,如果以后给账户/分类新增字段,容易忘记把新字段也编码进对应的 revision fields。** → 这是一个需要在代码评审时留意的手工纪律问题,不是这次能靠机制彻底消除的;影响也有限(忘记的字段变化不会让 revision 失效,只是少一道"提前量"报错,活查兜底依然有效)。
- **[权衡] `confirmation_token` 过期时间硬编码 15 分钟,不做成环境变量。** → 主动接受,和项目"不为用不上的灵活性加配置项"的一贯做法一致;真的需要调整时再改常量、发新版本。
- **[权衡] 分类彻底去掉 `type`,以后如果真需要"这个分类是收入还是支出"这种语义,要重新设计并做一次新的 schema 迁移。** → 主动接受,现在这个字段没有任何业务逻辑依赖它,留着就是纯粹的历史包袱;真有需求时再基于那个需求设计,好过现在猜一个用不上的字段。

## Migration Plan

没有生产部署,本地也没有真实存在的 `tally.db`。部署这次改动就是:编译新二进制,如果本地有旧结构的开发用 `tally.db`,直接删掉(和新表结构不兼容,`CREATE TABLE IF NOT EXISTS` 不会自动迁移旧数据),启动时会用新 schema 重新建表。同时需要在启动环境里新增 `TALLY_CONFIRMATION_SECRET`,否则进程会在启动时 fail-fast。没有数据迁移步骤;回滚就是换回旧二进制、旧 schema 文件。
