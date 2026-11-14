# Spec Delta

## Purpose

让调用方用一次 preview → confirm 往返,把同一账本内多笔交易的引用字段改成同一个新值,而不是对每笔交易重复调用 `update_transaction`。

## ADDED Requirements

### Requirement: 批量更新只改引用字段
`batch_update_transactions` SHALL 只接受对 `category_id`、`source_id`、`budget_id` 这三个字段的修改,且一次调用至少修改其中一个。它 SHALL NOT 接受对 `amount`、`currency`、`time`、`type`、`merchant`、`comment` 的修改;对这些字段的批量改写没有"一个值适用于多笔"的语义,调用方必须逐笔用 `update_transaction` 处理。

#### Scenario: 只改预算
- **WHEN** 调用 `batch_update_transactions`,patch 中只提供 `budget_id`
- **THEN** 命中的交易只有预算被改为新值,其余字段保持不变

#### Scenario: 同时改分类与来源
- **WHEN** 调用 `batch_update_transactions`,patch 中同时提供 `category_id` 与 `source_id`
- **THEN** 命中的交易的分类与来源都被改为新值,预算与其余字段保持不变

#### Scenario: patch 为空
- **WHEN** 调用 `batch_update_transactions`,patch 中 `category_id`、`source_id`、`budget_id` 都未提供
- **THEN** 请求被拒绝,返回说明至少要修改一个字段的错误,不修改任何交易

#### Scenario: 试图批量修改金额
- **WHEN** 调用 `batch_update_transactions` 的请求中包含对 `amount` 的修改
- **THEN** 请求被拒绝,返回说明该字段不支持批量修改的错误,不修改任何交易

### Requirement: 用筛选条件或交易 id 指定要更新的交易
`batch_update_transactions` SHALL 接受两种方式指定目标交易,二者取其一:一组交易 id,或一组筛选条件。筛选条件可包含 `category_id`、`source_id`、`budget_id`、`start_time`、`end_time` 中的一个或多个,多个条件以 AND 组合,且 `budget_id` SHALL 允许取表示"尚未分配预算"的值,以便找出迁移后还没回填的交易。所有目标交易 MUST 属于提供的 `ledger_id`。

#### Scenario: 按分类筛选
- **WHEN** 调用 `batch_update_transactions`,筛选条件只提供某个 `category_id`
- **THEN** 目标集合是该账本内分类为该值的全部交易,不包含其他账本的交易

#### Scenario: 按尚未分配预算筛选
- **WHEN** 调用 `batch_update_transactions`,筛选条件的 `budget_id` 取"尚未分配预算"的值
- **THEN** 目标集合是该账本内还没有分配预算的全部交易

#### Scenario: 指定的交易 id 不属于该账本
- **WHEN** 调用 `batch_update_transactions`,提供的某个交易 id 存在,但不属于提供的 `ledger_id`
- **THEN** 该 id 在预览结果中报告为未找到,不进入待更新集合

### Requirement: 预览一批更新
不带 `confirmation_token` 调用 `batch_update_transactions` 时,SHALL 解析出目标交易集合,对每笔返回其将被修改的字段的当前值与新值,并给出命中笔数,且不修改任何内容。返回结果 SHALL 包含一个覆盖整批预览内容的 confirmation token。新值引用的分类、来源或预算 MUST 存在且属于同一账本,否则预览即被拒绝。

#### Scenario: 预览一次按分类改预算
- **WHEN** 调用 `batch_update_transactions` 不带 `confirmation_token`,筛选某分类下的交易并把预算改为一个同账本内已存在的预算
- **THEN** 返回命中笔数、每笔交易预算字段的当前值与新值,以及一个 `confirmation_token` 及其过期时间,且没有任何交易被修改

#### Scenario: 新值引用的预算不存在
- **WHEN** 调用 `batch_update_transactions` 预览,patch 中的 `budget_id` 在该账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,不签发 `confirmation_token`

#### Scenario: 新值引用的分类属于另一个账本
- **WHEN** 调用 `batch_update_transactions` 预览,patch 中的 `category_id` 存在但属于另一个账本
- **THEN** 请求被拒绝,返回说明引用无效的错误,不签发 `confirmation_token`

### Requirement: 确认时逐笔尽力而为地更新已预览的批次
带着此前 preview 得到的 `confirmation_token` 调用 `batch_update_transactions` 时,SHALL 对该 token 覆盖的每笔交易:如果它在同一账本内仍然存在且自预览以来未变化,则按预览的新值更新;否则报告该项的结果,且不影响批次中其余项的处理。批次中某一项已被删除或自预览后被修改,SHALL NOT 阻止同一批次内其余项被更新。

#### Scenario: 确认前有一项已被修改
- **WHEN** 对一个已预览的批次调用确认,其中一笔交易在预览之后、确认之前被 `update_transaction` 修改过
- **THEN** 返回结果更新了批次中未变化的项,并将被修改过的那笔报告为已变化、未更新,不影响其余项

#### Scenario: 确认前有一项已被删除
- **WHEN** 对一个已预览的批次调用确认,其中一笔交易在预览之后、确认之前已经被删除
- **THEN** 返回结果更新了其余项,并将已不存在的那笔报告为未找到,而不是拒绝整个确认调用

#### Scenario: 用过期或被篡改的 token 确认
- **WHEN** 调用 `batch_update_transactions` 时所带的 `confirmation_token` 已过期,或其签名校验不通过
- **THEN** 该调用被拒绝并返回错误,批次中没有任何一笔交易被修改

### Requirement: 批量大小有上限
`batch_update_transactions` SHALL 在进行任何查找或更新之前,如果目标交易数量超过固定的批量上限,就直接拒绝整个调用。

#### Scenario: 批次超过最大数量
- **WHEN** 以会命中超过固定批量上限笔数的筛选条件调用 `batch_update_transactions` 进行预览
- **THEN** 该调用被拒绝并返回说明该上限的错误,且不发生任何更新
