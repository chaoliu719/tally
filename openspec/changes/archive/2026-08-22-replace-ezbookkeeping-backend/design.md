## Context

动机见 [proposal.md](proposal.md) 的 Why,这里只写"新的存储与业务逻辑层具体怎么搭"。

现状(`internal/tools/{accounts,categories,transactions}.go`)调用的是 ezbookkeeping 的 service 层(`services.Accounts`/`services.TransactionCategories`/`services.Transactions`),数据库由 `pkg/datastore` 负责初始化和迁移,所有查询按 `internal/bootstrap/user.go` 里 bootstrap 出来的一个假 `uid` 过滤。这次要换掉的就是这一整套——service 层、datastore/迁移机制、用户表和它的 bootstrap 逻辑。`internal/mcpserver` 和 `internal/authn`(HTTP 传输、bearer token 认证、JSON-RPC 分发、工具注册)不受影响,不在这次范围内。

## Goals / Non-Goals

**Goals:**
- 现有七个工具(`list_accounts`/`manage_account`/`list_categories`/`manage_category`/`create_transaction`/`get_transaction`/`search_transactions`)对调用方而言行为跟现在一样,只有一处主动声明的例外(见下面"交易可见性"这条决策)
- schema 和查询层完全由 tally 自己拥有,没有任何需要从别人源码里反查的隐藏规则
- 每一个设计选择都能追溯到 tally 自己的真实需求,不为 tally 已经排除的场景(多用户、子账户、模板、多实例部署)背负 ezbookkeeping 的历史包袱

**Non-Goals:**
- 账户/分类的更新与删除(下一个 change)
- 转账类型交易、汇率(不在范围内;也不为它们预留 schema 空间,理由见下面的决策)
- 任何增量式 schema 迁移机制——只有一个 schema 版本,靠 `CREATE TABLE IF NOT EXISTS` 一次性建好
- 把 `balance_adjustment` 暴露成 agent 可主动调用的操作(下一个 change 做;这次只在 `manage_account` 内部用)

## Decisions

### 数据访问:sqlc,不用 ORM

手写 SQL 放在 `internal/store/queries.sql`,由 `sqlc` 在构建期编译成类型安全的 Go 代码(生成的代码直接提交进仓库,不是运行时生成)。`internal/store/schema.sql` 是 DDL 的 source of truth,通过 `go:embed` 嵌进二进制,启动时执行 `CREATE TABLE IF NOT EXISTS` 建表——没有"从旧版本升级"这回事,所以不引入任何迁移框架。

驱动:`modernc.org/sqlite`(纯 Go,不需要 cgo)——`go build` 不需要 C 工具链,交叉编译也简单。代价是比 `mattn/go-sqlite3` 吞吐量稍低一些,但跟 tally 的实际规模完全不沾边。

并发:连接上开 `PRAGMA journal_mode=WAL` + `PRAGMA busy_timeout=<几秒>`。WAL 让读和写互不阻塞;`busy_timeout` 让写和写之间的冲突变成"等一下重试"而不是立刻报错。不额外加进程内的锁(比如包住所有写操作的 `sync.Mutex`)——tally 的写入量就是一个人记账,远远用不上这层保险。

**这条是硬性规则,不是权衡**:任何一次工具调用只要涉及一条以上的写入(目前只有 `manage_account` 带非零初始余额的情况,需要同时插入账户行和一笔 `balance_adjustment` 交易行),必须包在同一个 SQL 事务里。配合 WAL 的快照读,这能保证并发的 `list_accounts` 绝不会看到"账户已建好但对应的余额交易还没写"这种中间状态——因为余额是现算的(见下一条决策),这种中间状态会直接表现成一个错误的余额。

### Schema

不设任何 `uid` 列——整个数据库就是隐式的唯一账本,跟 tally 单用户的定位一致(`proposal.md` 里提到,以后真要做多用户,会是完全不同的认证方式,不是在现在这套表上加个 `uid` 过滤)。这也让 `internal/bootstrap/user.go` 里那套 bootstrap 假用户的逻辑整个消失。

```sql
CREATE TABLE IF NOT EXISTS accounts (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,              -- cash / checking_account / credit_card / virtual /
                                             -- debt / receivables / investment /
                                             -- savings_account / certificate_of_deposit
    currency    TEXT NOT NULL,              -- ISO 4217 代码;插入前在 Go 代码里对着静态币种表校验,
                                             -- 不是数据库约束
    comment     TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,           -- unix 秒
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,              -- income / expense / transfer
    parent_id   INTEGER NOT NULL DEFAULT 0, -- 0 = 一级分类;否则是某个一级分类的 id
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
    id           INTEGER PRIMARY KEY,
    type         TEXT NOT NULL,             -- income / expense / balance_adjustment
    account_id   INTEGER NOT NULL REFERENCES accounts(id),
    category_id  INTEGER REFERENCES categories(id),  -- balance_adjustment 时为 NULL
    amount       INTEGER NOT NULL,          -- 带符号,单位是该账户币种的最小单位
    time         INTEGER NOT NULL,          -- unix 秒
    comment      TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    CHECK (
        (type IN ('income', 'expense') AND category_id IS NOT NULL) OR
        (type = 'balance_adjustment' AND category_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_transactions_account_time  ON transactions(account_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_category_time ON transactions(category_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_time           ON transactions(time);
```

主键用 SQLite 原生自增的 `INTEGER PRIMARY KEY`——不用类 Snowflake 的生成器(那是为了解决多实例部署下的全局唯一性问题,单机单进程用不上)。线上协议仍然把 id 编码成十进制字符串(约定不变,避免 MCP 客户端那边 JSON number 精度丢失)。

不预留 `related_account_id` 之类给转账用的列。转账需要处理跨币种汇率,这部分完全没设计过;现在猜一个列结构,冒的风险是以后真正设计汇率时发现猜错了,换来的好处只是省一次 `ALTER TABLE`——不划算,以后真做的时候走一次有依据的迁移更好。

### 余额:现算,不存

`accounts` 表没有 `balance` 列。余额是查询时对 `transactions WHERE account_id = ?` 做 `SUM(amount)`(账户还没有交易时 `COALESCE(..., 0)`)。这是结构性保证,不是靠维护出来的不变量——因为压根不存在第二份可能跟交易流水对不上的事实。以 tally 的规模(单用户、本地 SQLite),这个聚合查询的开销可以忽略;`idx_transactions_account_time` 这个索引能让它保持在索引范围扫描,而不是全表扫描。

### `balance_adjustment`:带符号的增量,不是绝对目标值

ezbookkeeping 对应的概念(`modify_balance`)存的是一个绝对目标余额,而且只能在账户完全没有其他交易时插入一次——这个限制存在,是因为它的余额是一个缓存列,需要一个没有歧义的起点。既然 tally 的余额永远是现算的,这个限制在这里没有存在的理由:`balance_adjustment.amount` 就是一个带符号的增量,跟其他交易一样直接参与同一个 `SUM(amount)`,账户历史中任何时刻都能插入一笔,没有"必须为空"这个前提。这次 `manage_account` 内部用它(非零初始余额会变成一笔与账户创建同一个事务里写入的 `balance_adjustment`,金额等于该初始值)。把它作为 `create_transaction` 的一种可选 `type` 暴露出去、让 agent 以后能直接调用来修正余额,留给账户/分类更新与删除那个 change。

### 交易可见性:不做隐藏(主动偏离功能对等的一处)

现在 `search_transactions` 会把 `modify_balance` 类型的交易过滤掉(`internal/tools/transactions.go:229-232`),但 `get_transaction` 不会——这个不对称是因为 ezbookkeeping 把余额调整类的记录当成内部管道,不当正常交易。新实现去掉这个过滤:`search_transactions` 和 `get_transaction` 对 `balance_adjustment` 一视同仁。这是整次"功能对等迁移"里唯一一处主动的行为改动(已经在 `proposal.md` 的 What Changes 里点出来了)——没有缓存余额的叙事需要靠"隐藏它"去维护,也没有统计聚合功能需要靠隐藏它来保证准确性。

### 金额怎么存:带符号存储,按币种感知最小单位

`transactions.amount` **带符号**存储:`income` 为正,`expense` 为负,`balance_adjustment` 正负都可能。这样 `SUM(amount)` 直接就是余额,不需要额外的 `CASE` 逻辑。`income`/`expense` 的线上格式不变——`CreateTransactionInput.Amount` 仍然是必填的正数,方向由 `type` 决定(Go 代码里校验 `> 0`,写库前给 `expense` 取负);输出的 `TransactionInfo.Amount` 对这两种类型也仍然是正数(存储值的绝对值),跟现在测试过的行为完全一致。`balance_adjustment` 的 `TransactionInfo.Amount` 则直接输出带符号的值——这个类型本来就不在现有的、测试过的线上契约里(以前是完全不可见的内部管道),没有需要保持一致的旧行为。

最小单位改成真正按 ISO 4217 感知小数位数(比如日元/韩元 0 位,巴林第纳尔/科威特第纳尔/阿曼里亚尔 3 位),不再是 ezbookkeeping 那种不管什么币种都固定按两位小数换算的简化方式(查过它的 `pkg/core/currency.go` 和参考实现的 `decimalToMinor`,两边都没有任何按币种区分小数位数的数据,证实了这一点)。这就是 `proposal.md` 的 What Changes 里标了 **BREAKING** 的那处改动,不过因为目前没有任何真实账本数据,实际影响为零。

### 币种参考数据:一张静态 Go 表,兼两个用途

一份手工维护、对齐真实 ISO 4217 标准的 `map[string]int`(币种代码 → 小数位数),取代 ezbookkeeping 的 `validators.AllCurrencyNames`。一个代码在不在这张表里,就是"是否支持"的校验(`manage_account`"币种不受支持"那个场景);它对应的值就是上面小数位换算用的小数位数。具体怎么填这张表、维护它,是实现阶段(tasks.md)的事,不需要更多设计决策。它是 Go 里的静态数据,不是数据库表——运行时不会变,也不需要用 SQL 查它。

### 不做任何时区处理

`time` 字段(不管是 `create_transaction` 的输入还是任何工具的输出)一直就是纯 Unix 秒数,线上格式从来没暴露过时区/偏移信息。ezbookkeeping 每笔交易带的 `TimezoneUtcOffset` 纯粹是它自己 service 方法签名要求的内部管道(靠 `clientTimezone *time.Location` 参数算出来的),从来不属于 tally 的对外契约。这次直接去掉,不做任何替代——不是简化成一个 server 级配置,是彻底删除,因为工具接口这一层本来就没有任何东西需要"简化"。

### 不要 `TALLY_DEFAULT_CURRENCY`

它唯一的消费者(`internal/bootstrap/user.go` 里的 `EnsureSingleUser`)会随着用户表一起删掉。`manage_account` 的 `currency` 参数仍然维持必填,跟现在一样;工具接口的其他地方从来没有读过这个配置值。

## Risks / Trade-offs

- **[风险] 用聚合查询算余额,交易量大了会不会变慢。** → 对一本个人账本来说,离"多到会有影响"还差很多年;`idx_transactions_account_time` 这个索引保证它是索引扫描,不是全表扫描。这次不做进一步优化,真的观察到问题再说。
- **[风险] 手工维护的 ISO 4217 表跟真实标准之间会有漂移(新增货币偶尔发生,几乎不会有删除)。** → 频率低、代价小;一个错的/缺的代码只会让建账户时报一个清晰的错误,不会污染数据。这次不为它搭配套的自动同步机制。
- **[权衡] `search_transactions` 现在会搜出以前被隐藏的 `balance_adjustment` 记录。** → 主动接受(见"交易可见性"那条决策);如果 agent 自己拿 `search_transactions` 的结果去做 `SUM`(而不是信任账户自带的余额),现在会把这些记录也算进去,这只会更准确,不会更错。
- **[权衡] 不给转账预留列,以后要做真的要走一次 `ALTER TABLE`。** → 主动接受;另一个选项(在汇率方案都没设计出来之前先猜一个列结构)冒着"提前建错东西"的风险,划不来。

## Migration Plan

严格意义上不适用——没有生产部署,本地也没有真实存在的 `tally.db`(这个模式在 `.gitignore` 里,而且本来就不存在)。部署这次改动就是:编译新二进制、指向一个全新的 SQLite 路径(如果本地有开发用的 `tally.db`,它和新表结构不兼容,直接删掉即可)、启动。没有数据迁移步骤;真要回滚,就是换回旧二进制,配合另外准备的一个旧 schema 的文件。
