# Design

## Context

交易表目前只有 `comment` 一个自由文本字段,`create_transaction` / `update_transaction` / `search_transactions`、时间线 widget、`record` / `query` / `export` 三个 skill 都围绕「comment = 商户原文 + 明细」建立约定。线上已有一个带真实数据的 SQLite 数据库;项目没有迁移框架,`schema.sql` 只在启动时用 `CREATE TABLE IF NOT EXISTS` 执行,不会给已存在的表补列。参见 `proposal.md` 了解动机,`specs/` 了解行为契约。

## Goals / Non-Goals

**Goals:**

- 新增可空的 `merchant` 字段,与 `comment` 在存储、工具入参/出参、检索、widget、skill 五层都保持独立。
- 线上既有数据库原地升级,历史行不迁移、不改写、不丢失。
- keyword 一次检索同时覆盖 `merchant` 与 `comment`,让按商户名查先例/价格对比继续可用。

**Non-Goals:**

- 不做历史数据回填或 `comment` 拆分脚本(用户已明确只影响新记录)。
- 不做商户名的归一化、别名合并、去重或独立商户实体表。
- 不为 merchant 新增独立的检索参数、索引或分析维度;keyword + 现有汇总工具足够。
- 不改导出 CSV 的 7 列结构,merchant 不进入导出文件。

## Decisions

### 1. `merchant` 作为 transactions 上的普通 TEXT 列

`merchant TEXT NOT NULL DEFAULT ''`,与 `comment` 同级,不建独立表、不做外键。理由:商户名是每笔交易的描述性属性,不是需要跨笔复用的实体;独立表会引入「同一个商户多个 id」「改名影响历史」的治理问题,而当前需求只是把两段文本分开。空字符串表示「没有商户」,与 `comment` 现有的可选语义一致。

**替代方案**:新建 `merchants` 表、交易引用 `merchant_id`。被否:需要商户消歧、合并、命名治理,远超本次范围。

### 2. 启动时做幂等 `ALTER TABLE`,而不是删库重建

在 `store.Open()` 执行完 `schema.sql` 后,用 `PRAGMA table_info(transactions)` 检查 `merchant` 是否存在;不存在才执行 `ALTER TABLE transactions ADD COLUMN merchant TEXT NOT NULL DEFAULT ''`。理由:线上有真实数据,且 `CREATE TABLE IF NOT EXISTS` 对已存在的表是空操作。该迁移只增列、不读写行、可重复执行。

**替代方案**:引入 `user_version` 迁移框架。被否:当前只有一个 schema 版本、只有一次加法迁移,引入框架的复杂度高于收益;若将来出现更多迁移再升级。

### 3. keyword 用 OR 同时匹配两列

`SearchTransactions` / `SearchTransactionsDesc` 的 keyword 条件改为 `(LOWER(comment) LIKE ... OR LOWER(merchant) LIKE ...)`,两列共用同一个已转义的 keyword。理由:同一关键词在两个字段的语义都是「这段文本里出现过」,OR 是最小改动;老记录靠 comment 仍可命中,新记录靠 merchant 或 comment 都能命中。

**替代方案**:新增 `merchant` 过滤参数。被否:调用方(Agent)需要判断该传哪个字段,而用户的自然语言查询本来就是模糊的;关键词覆盖两列更简单。

### 4. `update_transaction` 保持全量替换,merchant 参与替换

不改成 partial patch,merchant 与 comment 一样:调用时不传就存空。理由是维持既有工具契约,避免同一工具出现两种更新语义。代价是漏传会静默清空——通过 `plugin-record-skill` 的「先读后写」纪律和单测覆盖来防。

### 5. 历史数据不动

旧行的 `merchant` 为空,`comment` 保持原样。理由:历史 comment 是「商户名 + 明细」的混合文本,自动拆分既无法保证准确,也会破坏用户已经认可的原文留痕;用户已明确选择只影响新记录。

### 6. widget 行内分层展示

`merchant` > `comment` > 分类名 > `#id` 作为主标题的优先级,`comment` 与分类名作为副行。理由:商户名是浏览流水时最先用来识别一笔交易的信息;备注是补充说明。历史行没有 merchant 时自然回退到 comment,不需要数据迁移。

## Risks / Trade-offs

- **[老库缺列导致启动失败]** → 启动时先 `PRAGMA table_info` 再决定是否 `ALTER TABLE`;迁移测试用旧 schema 建库、插入数据、调用 `Open()` 验证行数与内容不变。
- **[`update_transaction` 漏传 merchant 静默清空]** → 与 comment 同一类已知陷阱;skill 明确要求先 `get_transaction` 再全量提交,单测覆盖「漏传后为空」的行为。
- **[LIKE 两列全表扫描]** → 个人账本量级(万级以内)可接受;keyword 本来就没有索引,不因本次变更恶化。
- **[历史行 merchant 为空,按商户聚合会比新数据少]** → 明确接受;老数据仍可通过 keyword 搜到,只是不做商户维度回填。
- **[sqlc 生成代码与手写代码不一致]** → 修改 `schema.sql` / `queries.sql` 后统一运行 `sqlc generate`,CI 的 build/test 会兜底。
- **[回滚到旧镜像]** → 旧镜像的查询显式列出列名,不会受多出来的 `merchant` 列影响;数据库可保持新结构,回滚是安全的。

## Migration Plan

1. 合并代码后打 `v*` tag,触发 GHCR 镜像构建。
2. 线上 `docker compose pull && docker compose up -d` 重启;启动时自动完成 `ALTER TABLE`。
3. 验证:启动日志无 schema 错误;`get_transaction` 能读到老交易且 `merchant` 为空;新建一笔带 merchant 的交易后 `search_transactions(keyword=<商户名>)` 能命中。
4. 回滚:切回上一个镜像即可,数据库不需要回退(多出的列被旧查询忽略)。

## Open Questions

（无）
