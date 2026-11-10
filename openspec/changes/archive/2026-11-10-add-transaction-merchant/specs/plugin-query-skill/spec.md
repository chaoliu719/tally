# Spec Delta

## MODIFIED Requirements

### Requirement: 检索某商户或关键词的历史交易
Agent SHALL 使用 `search_transactions(ledger_id, keyword=<关键词>)` 按 `merchant` 或 `comment` 的子串匹配检索历史交易;`keyword` 大小写不敏感且不支持通配符,`%`/`_` 按字面字符匹配。需要某一笔的完整字段时 Agent SHALL 调用 `get_transaction(id)` 取全量,不只依赖搜索结果摘要。

#### Scenario: 用户问某商户/某笔的历史或明细
- **WHEN** 用户想查看某商户或某关键词相关的历史交易,或某一笔的完整信息
- **THEN** agent 用 `search_transactions` 按关键词检索并按时间排序呈现;需要单笔完整字段时用 `get_transaction(id)` 取全量

#### Scenario: 关键词命中商户名而非备注
- **WHEN** 用户按商户名查询,该商户名保存在历史交易的 `merchant` 字段里,`comment` 并不包含它
- **THEN** `search_transactions` 仍返回这些交易,agent 正常按时间呈现

#### Scenario: 关键词含通配符字符
- **WHEN** agent 为放宽匹配而想在 `keyword` 里加入 `%` 或 `_`
- **THEN** agent 不加通配符,改用更短或更核心的词放宽、用更多字收窄,因为这些符号会被当作字面字符匹配

### Requirement: 正确解读返回值的金额、币种与 comment 语义
Agent SHALL 按以下语义解读查询返回值:`amount` 的符号与收支方向按 tally 既有数据模型理解,不默认为支出;多币种结果分币种分别呈现,不按汇率相加或合并;`merchant` 是这笔钱付给谁 / 谁付的钱的原名,`comment` 是用户自己的备注,二者是独立字段,回答「这笔是什么」时 SHALL 先呈现 `merchant`(为空时省略),再呈现 `comment`,最后补充分类、来源等结构化字段。

#### Scenario: 结果含多种币种
- **WHEN** 一段时间的交易清单里存在多种币种
- **THEN** agent 分币种分别列出,不将不同币种金额相加或换算合并

#### Scenario: 用户问某笔交易是什么
- **WHEN** 用户询问某笔交易的性质
- **THEN** agent 先呈现 `merchant`(若为空则跳过),再呈现 `comment`,最后补充分类/来源等结构化字段,不只用分类名作答

#### Scenario: 商户名与备注同时存在
- **WHEN** 一笔交易的 `merchant` 与 `comment` 都有值
- **THEN** agent 把它们作为两个独立信息分别呈现,不拼接成一段、不互相覆盖

#### Scenario: 历史交易没有商户名
- **WHEN** 一笔历史交易的 `merchant` 为空、但 `comment` 有值
- **THEN** agent 正常呈现 `comment`,不因缺少商户名而留空或报错
