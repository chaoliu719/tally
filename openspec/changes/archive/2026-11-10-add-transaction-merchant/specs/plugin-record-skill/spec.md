# Spec Delta

## ADDED Requirements

### Requirement: 商户名与备注分开写入
Agent SHALL 把「这笔钱付给了谁 / 谁付的钱」写入 `merchant`,把用户自己想补充的话写入 `comment`。`merchant` SHALL 是银行账单或用户口述中的商户原名,原样写入,不做归一化、缩写、大小写整理或别名替换;没有商户的交易 SHALL 允许 `merchant` 为空。`comment` SHALL NOT 再被要求承载商户名,只在用户提供了想记的内容时才写入。

#### Scenario: 用户提供商户与自己的说明
- **WHEN** 用户说"星巴克 32 块,和同事下午茶",准备记录一笔支出
- **THEN** Agent 把"星巴克"写入 `merchant`,把"和同事下午茶"写入 `comment`,分别放入对话预览

#### Scenario: 银行账单里的商户原名
- **WHEN** 银行流水上的商户名是 `STARBUCKS #4821 TAIPEI`,用户没有额外说明
- **THEN** Agent 原样把该字符串写入 `merchant`,`comment` 留空,不改写成"星巴克"或其他整理后的名字

#### Scenario: 没有商户的交易
- **WHEN** 一笔现金或个人往来交易没有可识别的商户
- **THEN** Agent 不编造 `merchant`,该字段留空,只按用户提供的内容写 `comment`

## MODIFIED Requirements

### Requirement: 记录前必须检索先例
Agent SHALL 在为一笔新交易选择来源与分类之前,先用 `search_transactions` 按商户/关键词检索该账本内 `merchant` 或 `comment` 命中的历史交易,并把命中先例的 `source_id`/`category_id` 作为默认选择。

#### Scenario: 存在同关键词历史交易
- **WHEN** 该账本内存在 `merchant` 或 `comment` 命中同商户/关键词的历史交易
- **THEN** agent 使用这些历史交易的 `source_id`/`category_id` 作为默认选择,不要求用户重新提供

#### Scenario: 先例只命中商户名
- **WHEN** 历史交易的 `merchant` 命中关键词,`comment` 没有命中
- **THEN** agent 仍然把该交易视为先例,采用它的 `source_id`/`category_id`

#### Scenario: 未搜到先例
- **WHEN** 关键词检索没有命中任何历史交易
- **THEN** agent 向用户询问来源与分类,而不是凭空猜测

### Requirement: 写入前必须给出对话预览
Agent SHALL 在调用 `create_transaction` 或 `update_transaction` 之前,先在对话中给出包含全部字段的结构化预览,等待用户确认或修改。预览 SHALL 把 `merchant` 与 `comment` 分两行展示,不得合并成一段文本。

#### Scenario: 预览包含完整字段
- **WHEN** agent 已经确定好一笔新交易的金额、来源、分类、时间、商户名与备注,准备写入
- **THEN** agent 先展示这些字段的结构化预览(商户名与备注分别列出),用户确认后才调用 `create_transaction`

#### Scenario: 预览中的商户名与备注
- **WHEN** 一笔交易同时有 `merchant` 与 `comment`
- **THEN** 预览分别标明"商户/收款方"和"备注"两个字段的值,不让用户误以为它们会被合并保存

#### Scenario: 用户要求修改预览
- **WHEN** 用户在预览阶段要求修改某个字段
- **THEN** agent 更新预览内容并重新等待确认,不跳过确认直接写入

### Requirement: 修正交易前必须读出当前完整记录
Agent SHALL 在调用 `update_transaction` 之前,先用 `get_transaction` 获取该交易当前的完整字段值,并在 `update_transaction` 调用中带上全部未变更字段的原值,包括 `merchant` 与 `comment`。

#### Scenario: 调用 update_transaction 前
- **WHEN** agent 准备调用 `update_transaction` 修正一笔交易的任意字段
- **THEN** agent 先调用 `get_transaction` 读出当前完整记录,并把其余未变更的字段(包括 `merchant` 与 `comment`)原样带入本次调用,避免遗漏字段被服务端静默清空

#### Scenario: 只改分类但漏带商户名
- **WHEN** agent 因为分类判断错误而修正一笔有商户名的交易
- **THEN** agent 在调用前读出原 `merchant`,并在本次调用中原样带上,不用空值覆盖它

### Requirement: 跨品类消费应提议拆分为多笔
Agent SHALL 在记录一笔涵盖多个商品子类的商户消费(典型场景:超市、综合电商订单)时,于对话预览中提议按商品子类拆分为多笔交易,由用户在预览阶段决定是否拆分以及如何拆分。Agent SHALL NOT 在未经用户确认的情况下,自行把这类消费合并记为单一笼统分类。

#### Scenario: 一张小票涵盖多个品类
- **WHEN** 用户提供的一笔消费明细里含有分属不同分类的商品(如食材、日用、零食混在一次超市结账)
- **THEN** agent 在对话预览中给出按子类拆分的建议方案(每笔的金额、分类、来源、`merchant` 与 `comment`),询问用户是否按此拆分或如何调整

#### Scenario: 用户选择不拆分
- **WHEN** 用户在预览阶段表示这笔不需要拆分
- **THEN** agent 按用户指定的单一分类记为一笔,不再坚持拆分

#### Scenario: 拆分后逐笔写入
- **WHEN** 用户确认了拆分方案
- **THEN** agent 为每个子类分别调用 `create_transaction`,每笔带上相同的 `merchant` 与该子类对应的 `comment`

## REMOVED Requirements

### Requirement: comment 必须保留原始凭据文本
**Reason**: `merchant` 字段引入后,商户原名与用户备注分开保存,`comment` 不再承担"原文留痕"职责,而是用户自己的备注。
**Migration**: 新记录把商户原名写入 `merchant`、用户备注写入 `comment`;历史交易的 `comment` 不做迁移,原样保留。Agent 不再要求 `comment` 写入账单原文。

### Requirement: comment 同时承载商户原名与商品明细
**Reason**: 交易现在有独立的 `merchant` 字段,且 `search_transactions` 的 `keyword` 同时匹配 `merchant` 与 `comment`,不再需要把所有检索线索塞进 `comment`。
**Migration**: 新记录用 `merchant` 保存商户名,用 `comment` 保存用户想补充的备注或商品明细;历史交易保持原样,仍可被 keyword 通过 `comment` 命中。
