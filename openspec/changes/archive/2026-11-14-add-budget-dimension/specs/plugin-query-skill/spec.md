# Spec Delta

## ADDED Requirements

### Requirement: 查询结果呈现预算
Agent SHALL 在向用户呈现交易清单或单笔交易时带上它的预算。预算以名称呈现,不以 id 呈现。尚未分配预算的交易 SHALL 被明确标出,而不是省略或归入某个预算。

#### Scenario: 呈现一段时间的清单
- **WHEN** agent 按时间逐笔列出一段交易
- **THEN** 每笔都带上它的预算名称

#### Scenario: 清单中有尚未分配预算的交易
- **WHEN** 查询结果里有一笔还没有被分配预算的交易
- **THEN** agent 明确标出它尚未分配预算,不把它归进任何一个预算,也不略过不提

## MODIFIED Requirements

### Requirement: 正确解读返回值的金额、币种与 comment 语义
Agent SHALL 按以下语义解读查询返回值:`amount` 的符号与收支方向按 tally 既有数据模型理解,不默认为支出;多币种结果分币种分别呈现,不按汇率相加或合并;`merchant` 是这笔钱付给谁 / 谁付的钱的原名,`comment` 是用户自己的备注,二者是独立字段,回答「这笔是什么」时 SHALL 先呈现 `merchant`(为空时省略),再呈现 `comment`,最后补充分类、来源、预算等结构化字段;`budget_id` 是对预算的引用,呈现时 SHALL 用预算名称而不是这个 id。

#### Scenario: 结果含多种币种
- **WHEN** 一段时间的交易清单里存在多种币种
- **THEN** agent 分币种分别列出,不将不同币种金额相加或换算合并

#### Scenario: 用户问某笔交易是什么
- **WHEN** 用户询问某笔交易的性质
- **THEN** agent 先呈现 `merchant`(若为空则跳过),再呈现 `comment`,最后补充分类、来源、预算等结构化字段,不只用分类名作答

#### Scenario: 商户名与备注同时存在
- **WHEN** 一笔交易的 `merchant` 与 `comment` 都有值
- **THEN** agent 把它们作为两个独立信息分别呈现,不拼接成一段、不互相覆盖

#### Scenario: 历史交易没有商户名
- **WHEN** 一笔历史交易的 `merchant` 为空、但 `comment` 有值
- **THEN** agent 正常呈现 `comment`,不因缺少商户名而留空或报错
