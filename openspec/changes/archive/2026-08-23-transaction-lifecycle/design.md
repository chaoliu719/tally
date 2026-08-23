## Context

动机见 [proposal.md](proposal.md)。现状:`internal/tools/transactions.go` 只有 `createTransaction`/`getTransaction`/`searchTransactions`;`internal/confirm`(见 `internal/confirm/confirm.go`)已经提供了通用的、无状态的 `Issue`/`Verify` preview→apply 令牌机制,目前被 `internal/tools/accounts.go` 的 `deleteAccount` 和 `internal/tools/categories.go` 的 `deleteCategory` 使用;两者的 revision 都是"决定能否删除的门槛字段(引用计数)+ 其余用于探测预览后资源是否漂移的字段"的组合。`transactions` 表(`internal/store/schema.sql`)目前没有 `updated_at` 列——因为在这次之前交易从未被更新过。账户余额(`GetAccountBalance` 查询,`internal/store/queries.sql`)是对 `transactions.amount` 现算 `SUM`,不是存储字段。

项目级别的写操作安全原则(见 openspec/project.md 的"写操作安全机制"一节)明确:"只有批量操作和破坏性操作(比如批量改交易、删账户、删分类)要求 preview → apply 两步确认;单条记录的增/改直接执行,不强制二次确认"。这条原则把"删"整体归入需要确认的破坏性操作类别、把单条"增/改"整体归入不需要确认的类别——这次的 `update_transaction`/`delete_transaction` 直接落在这条既定原则划好的两侧,不需要重新评估。

## Goals / Non-Goals

**Goals:**
- 新增 `update_transaction`、`delete_transaction` 两个独立 MCP 工具
- `delete_transaction` 复用 `internal/confirm` 的 preview → apply 机制
- 验证(而非新增逻辑)删除/修改交易后账户余额通过 `SUM(amount)` 现算自动保持正确
- 打通"先用 `delete_transaction` 清空某账户/分类下的全部交易、再删除该账户/分类"这条路径

**Non-Goals:**
- 批量删除/批量修改交易的 preview/apply(留给后续 change)
- transfer 类型交易、汇率
- 修改 `deleteAccount`/`deleteCategory` 本身的判定逻辑——这次不改它们一个字符,只是让它们已有的"零引用才能删"路径第一次变得可达
- id 的线上编码格式

## Decisions

### `update_transaction`/`delete_transaction` 是独立工具,不是 `manage_transaction` 的 operation 分发

`manage_account`/`manage_category` 用 `operation` 字段分发 create/update/delete,是因为三种操作共享同一组输入字段(name/type/comment 等),用一个 struct、一个 switch 更省重复。但 `create_transaction`/`get_transaction`/`search_transactions` 从一开始就是三个独立工具,输入字段互不相同(create 要 type/amount,get 只要 id,search 是一组筛选条件)——这次新增的 update/delete 延续同一风格:`update_transaction` 的输入接近 `create_transaction`(多一个 `id`),`delete_transaction` 的输入只有 `id`/`confirmation_token`,和 `create_transaction`/`search_transactions` 的输入形状都不同。硬凑一个 `manage_transaction` + `operation` 不会减少重复,反而会让三种早已独立的输入形状挤进一个可选字段全开的大 struct。保留独立工具,和文件里已有的三个工具一致。

**备选方案**:`manage_transaction`(`operation=create/get/search/update/delete`)—— 放弃,理由如上。

### `update_transaction`:完整字段替换,复用 `create_transaction` 的校验规则

`update_transaction` 接受与 `CreateTransactionInput` 相同的字段集合(`type`/`account_id`/`category_id`/`amount`/`time`/`comment`)外加必需的 `id`,要求全部字段一起提供(不支持只改一个字段)。校验规则——income/expense 需要合法 `category_id` 且 `amount` 为正;`balance_adjustment` 不能带 `category_id` 且 `amount` 非零——与 `createTransaction` 完全一致,因此 `internal/tools/transactions.go` 里把 `createTransaction` 现有的字段校验部分提取成一个共享的校验函数(输入 `CreateTransactionInput` 形状的字段,输出规范化后的 `signedAmount`/`categoryID`),供 `createTransaction` 和 `updateTransaction` 共用,避免两处维护同一套 if/else。

允许修改 `account_id`(等价于把这笔交易改记到另一个账户)——这是修正记账错误(记错账户)的合理场景,不禁止;因为余额是查询时现算,两个账户的余额都会在下一次查询时自动反映变化,不需要额外的跨账户处理逻辑。这和真正的"转账"类型交易(需要同时产生一笔支出和一笔收入、可能跨币种)是不同概念,后者仍然留给未来的 transfer change。

不需要 `confirmation_token`:遵循 project.md 的既定原则,单条记录的"改"直接执行。

### `delete_transaction`:走 preview → apply,复用 `internal/confirm`

按 project.md 的既定原则,"删"整体归入需要确认的破坏性操作类别,`delete_transaction` 没有理由例外——交易记录是账本的核心历史数据,一旦删除无法恢复,风险性质和删账户、删分类(同样是不可逆的数据丢失)是一类的,只是波及面通常更小(一条记录而非可能级联影响一批历史)。这次不为"波及面更小"单独开一个不需要确认的例外:project.md 划线的依据是"是增改还是删",不是"影响几行数据"。

与账户/分类删除的关键区别:**没有前置门槛**。账户/分类删除只有在引用计数为零时才允许(账户/分类没有实际数据丢失以外的"内容",门槛是为了防止连带影响交易记录);交易本身没有下游引用者,任意已存在的交易都允许删除。这意味着:
- preview 阶段只做"交易是否存在"这一个检查,不需要额外的计数查询
- revision 覆盖交易的全部字段(`Type`/`AccountID`/`CategoryID`/`Amount`/`Time`/`Comment`),纯粹用于探测预览之后交易是否被 `update_transaction` 修改或被删除——不存在"决定能否删除"和"漂移检测"两类字段的划分,因为没有门槛字段
- confirm action 常量新增 `"delete_transaction"`,与 `"delete_account"`/`"delete_category"` 并列

apply 阶段仍然在同一个 `sql.Tx` 里,`Verify` 通过后先 `GetTransaction` 活查一次确认交易还存在,再执行 `DELETE`,和账户/分类删除的 apply 阶段"活查兜底"是同一道防线(参见 [account-category-lifecycle/design.md](../archive/2026-08-23-account-category-lifecycle/design.md) 的"apply 时的活查兜底,与 revision 校验是两道独立防线")。

### `transactions` 表新增 `updated_at` 列

`accounts`/`categories` 两张已经支持更新的表都有 `updated_at`,且都是"内部维护、不通过 API 暴露"的模式(`AccountInfo`/`CategoryInfo` 都不包含 `updated_at` 字段)。这次让 `transactions` 跟随同一约定:新增 `updated_at INTEGER NOT NULL`,`create_transaction` 时设为等于 `created_at`,`update_transaction` 时设为 `time.Now().Unix()`;`TransactionInfo` 不新增字段暴露它,和 `created_at`(同样存在于表里但从未通过 `TransactionInfo` 暴露)保持一致。

**备选方案**:不加这一列,因为目前没有任何读路径需要它——放弃,理由是这个"内部维护但不暴露"的模式已经在 `accounts`/`categories` 上出现过两次,是项目里对"可变数据表"的既定约定,而不是为假设的未来需求新增字段;补齐这一列的成本(一列 + create/update 两处赋值)很低。

## Risks / Trade-offs

- **[权衡] 单条交易删除也要求两步确认,可能显得繁琐。** → 主动接受,和 project.md 明确划线的"删=需要确认"原则保持一致,不为这次的资源类型开例外;MCP 客户端自身通常也会有一层调用确认 UI。
- **[风险] `update_transaction` 允许改 `account_id`,如果调用方本意是想做"转账",可能会错误地用两次 `update_transaction`/`create_transaction`拼出一个手工转账,而不是等未来的 transfer 类型交易。** → 不做额外限制;这属于调用方(通常是 Claude 通过 MCP 调用)对语义的理解问题,不是这次要解决的范围,后续 transfer change 落地后可以在工具描述里补充引导。
- **[风险] revision 覆盖交易的全部字段,如果以后给 transaction 加新字段容易忘记同步进 revision。** → 与 account-category-lifecycle 的同类风险一致,是代码评审阶段需要留意的手工纪律问题;影响有限(活查兜底依然生效)。

## Migration Plan

没有生产部署,本地也没有真实 `tally.db` 数据。部署这次改动就是:编译新二进制,本地如果有旧结构的开发用 `tally.db` 直接删掉,启动时用新 schema(多了 `transactions.updated_at` 列)重新建表。没有数据迁移步骤;回滚就是换回旧二进制、旧 schema 文件。
