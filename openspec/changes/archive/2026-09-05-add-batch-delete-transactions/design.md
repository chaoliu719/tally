## Context

`delete_transaction`（`mcp/internal/tools/transactions.go`）是一对 preview→apply 流程，背后由 `mcp/internal/confirm` 支撑：`Issue`/`Verify` 对一个 `(action, id, revision)` 三元组签名并校验，其中 `revision` 是 `transactionDeletionRevision(t)` —— 对交易可变字段的哈希，仅用于检测该行自预览以来是否发生变化（交易删除没有引用计数门槛，这点和来源/分类删除不同）。动机见 proposal.md - Why（批量删多笔交易现在每笔都要两次工具调用，而且跨多轮分开处理容易出错）。

## Goals / Non-Goals

- 目标：一次 preview 调用 + 一次 confirm 调用，删除同一账本内一组数量有上限的交易。
- 目标：批次里单独一项过期/被改/已消失，绝不阻塞其余项的删除（见 spec 的 best-effort 要求）。
- 目标：`delete_transaction` 现有的 token 格式、行为和测试完全不受影响。
- 非目标：批量创建/更新，或任何批量导入/解析流程 —— 按 proposal.md 排除在范围外。
- 非目标：跨账本批次 —— 一次批量调用限定在一个 `ledger_id` 内，和其他交易工具一致。
- 非目标：全有全无的原子语义 —— 明确选择了尽力而为（见 Decisions）。

## Decisions

### Token payload：新增一种批量 payload，与现有单笔 payload 并存，而不是把单笔泛化
`confirm.payload` 只有单个 `ID`/`Revision`。批量需要一组 `(id, revision)`。与其扩宽现有 `payload` 结构（那会牵动 `Issue`/`Verify` 的所有调用方，包括来源/分类/交易的单笔删除），不如在 `mcp/internal/confirm` 里新增第二种 payload 形态，配上 `IssueBatch`/`VerifyBatch` 函数，复用已有的 `sign`/`splitToken`/base64 辅助函数。`Issue`/`Verify` 以及它们对单资源删除的线上格式保持不变。

考虑过的替代方案：复用 `Issue`/`Verify`，把所有 id+revision 哈希成一个合并的 `Revision` 字符串。放弃的原因是：best-effort 语义要求确认时逐项校验 revision（逐项判断"是否被删除"、逐项判断"是否已变化"）——合并成一个哈希会被迫变成全有全无的校验，违背了初衷。

### 批量 token 绑定预览时的确切 id 集合
`VerifyBatch` 在触碰任何行之前，除了校验签名和过期时间外，还会核对确认时传入的 id 列表和 token 里的 id 列表是否一致（不区分顺序）。这样可以防止有人拿着为 `[1,2,3]` 签发的 token，去确认一个不同或更大的集合 —— token 只授权它预览过的那一批，不多不少。

### 尽力而为的执行方式：每一项各自独立一个事务，而不是整批一个事务
确认阶段遍历 token 里的每个 `(id, revision)`：逐项重新查询、比对 revision，并在各自独立的数据库事务里删除（照搬 `deleteTransaction` 现有的 begin/check/delete/commit 流程，而不是引入新模式）。失败情况（未找到、revision 已变化）逐项记录，不会回滚或跳过其余项。这直接实现了 best-effort 要求，也正是让本次手动清理中真实遇到的情况（37 笔里有 2 笔在确认时已经不存在）变成一个正常的、可报告的结果，而不是一个错误。

考虑过的替代方案：把整批包在一个 SQL 事务里，全有全无地删除。按 proposal 里明确的 best-effort 选择放弃 —— 否则一个 30 项批次里有一项过期，就会迫使剩下 29 项重新走一遍 preview 和 confirm。

### 固定的批量上限：每次调用最多 100 个 id
选择这个数字是为了从容覆盖现实中手动清理和单次对话内批量操作的规模（本次任务是 37 笔），并留有余量，同时把 token payload 体积和单次调用的数据库开销限制在可控范围内。在预览调用一开始就校验，如果 `len(ids) > 100` 就在做任何查找之前直接拒绝整个调用。不做成可配置项，只有实际使用触及上限时才重新考虑。

### 复用而非重新实现现有的单笔逻辑
Handler 复用 `transactionDeletionRevision` 计算 revision，以及 `deleteTransaction` 已经在用的"查询/比对/删除"同一套顺序，逐项执行，这样批量删除和单笔删除对"什么算变化"、"怎么执行删除"不会产生分歧。

## Risks / Trade-offs

- 【token payload 比单笔删除更大】→ 用 100 项的上限限制住，仍然远在典型 MCP 消息大小限制之内。
- 【尽力而为意味着一次确认调用可能"成功返回"却一笔都没删（如果全部都过期了）】→ 返回结果逐项报告结果，调用方能看清这一点，并且只需针对需要处理的那些结果重新 preview，这和人工一笔笔重新执行单笔删除时看到的信息量是一样的，只是打包在了一起。
- 【`confirm` 包新增两个导出函数，增加了它的接口面】→ 刻意收窄在"批量删除"这一件事上，而不是为了假想中未来的批量创建/更新去提前泛化 `Issue`/`Verify`，这和项目对批量创建/更新的 YAGNI 立场是一致的。

## Migration Plan

纯新增性质：新工具、新的 `confirm` 函数，没有 schema 改动，不改变任何现有工具的行为。如果要回滚，只需要移除新增的工具/函数，没有其他顾虑。
