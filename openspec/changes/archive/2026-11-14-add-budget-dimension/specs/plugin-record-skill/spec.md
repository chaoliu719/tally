# Spec Delta

## ADDED Requirements

### Requirement: 记录前必须为交易选定预算
Agent SHALL 在记录一笔新交易时为其选定一个预算。预算的选择 SHALL 依据交易本身的性质(是否周期性且刚性不可省、是否计划内的大额、是否计划外不得不花,其余归 `日常`),而不是依据它的分类——分类回答"钱花在什么上",回答不了"这笔钱算哪一类"。性质无法判断时,Agent SHALL 依次参考同商户、同分类的历史交易落在哪个预算;仍无法确定、历史出现多个预算、或金额明显偏离该商户或分类的历史区间时,Agent SHALL 向用户询问,不得自行猜测或套用默认值。

#### Scenario: 性质明确的交易
- **WHEN** 一笔交易的性质可以从商户、备注或金额直接判断(如年度车位租赁费、话费、突发的医疗支出)
- **THEN** Agent 据此选定预算,不因该交易的分类而改判

#### Scenario: 性质不明但有唯一历史先例
- **WHEN** 一笔交易的性质无法直接判断,但同商户的历史交易全部落在同一个预算
- **THEN** Agent 采用该预算,并在预览中标明它来自历史先例

#### Scenario: 无法确定预算
- **WHEN** 一笔交易既无法从性质判断,又没有唯一的历史先例,或金额明显偏离其商户或分类的历史区间
- **THEN** Agent 向用户询问该用哪个预算,不自行猜测,也不填写默认值

### Requirement: 预览必须包含预算及其依据
Agent SHALL 在对话预览中给出所选预算,并标明这个选择的依据(来自性质判断,或来自哪类历史先例)。

#### Scenario: 预览展示预算
- **WHEN** Agent 准备写入一笔新交易
- **THEN** 对话预览中包含预算字段,并说明它是依据性质判断还是依据历史先例得出的

### Requirement: 修正交易时必须带上预算
Agent SHALL 在调用 `update_transaction` 时带上 `budget_id`。与 `merchant`、`comment` 不同,`budget_id` 省略会被服务端拒绝而不是静默清空,Agent SHALL 在修正任意字段时都原样带上当前的 `budget_id`,除非用户明确要求改预算。

#### Scenario: 只改分类
- **WHEN** Agent 因为分类判断错误而修正一笔交易,用户没有要求改预算
- **THEN** Agent 在调用中原样带上该交易当前的 `budget_id`,不省略它

## MODIFIED Requirements

### Requirement: 记录前必须检索先例
Agent SHALL 在为一笔新交易选择来源、分类与预算之前,先用 `search_transactions` 按商户/关键词检索该账本内 `merchant` 或 `comment` 命中的历史交易,并把命中先例的 `source_id`/`category_id`/`budget_id` 作为默认选择。

#### Scenario: 存在同关键词历史交易
- **WHEN** 该账本内存在 `merchant` 或 `comment` 命中同商户/关键词的历史交易
- **THEN** agent 使用这些历史交易的 `source_id`/`category_id`/`budget_id` 作为默认选择,不要求用户重新提供

#### Scenario: 先例只命中商户名
- **WHEN** 历史交易的 `merchant` 命中关键词,`comment` 没有命中
- **THEN** agent 仍然把该交易视为先例,采用它的 `source_id`/`category_id`/`budget_id`

#### Scenario: 未搜到先例
- **WHEN** 关键词检索没有命中任何历史交易
- **THEN** agent 向用户询问来源、分类与预算,而不是凭空猜测

### Requirement: 写入前必须给出对话预览
Agent SHALL 在调用 `create_transaction` 或 `update_transaction` 之前,先在对话中给出包含全部字段的结构化预览,等待用户确认或修改。预览 SHALL 包含预算,并 SHALL 把 `merchant` 与 `comment` 分两行展示,不得合并成一段文本。

#### Scenario: 预览包含完整字段
- **WHEN** agent 已经确定好一笔新交易的金额、来源、分类、预算、时间、商户名与备注,准备写入
- **THEN** agent 先展示这些字段的结构化预览(商户名与备注分别列出),用户确认后才调用 `create_transaction`

#### Scenario: 预览中的商户名与备注
- **WHEN** 一笔交易同时有 `merchant` 与 `comment`
- **THEN** 预览分别标明"商户/收款方"和"备注"两个字段的值,不让用户误以为它们会被合并保存

#### Scenario: 用户要求修改预览
- **WHEN** 用户在预览阶段要求修改某个字段
- **THEN** agent 更新预览内容并重新等待确认,不跳过确认直接写入

### Requirement: 修正交易前必须读出当前完整记录
Agent SHALL 在调用 `update_transaction` 之前,先用 `get_transaction` 获取该交易当前的完整字段值,并在 `update_transaction` 调用中带上全部未变更字段的原值,包括 `budget_id`、`merchant` 与 `comment`。

#### Scenario: 调用 update_transaction 前
- **WHEN** agent 准备调用 `update_transaction` 修正一笔交易的任意字段
- **THEN** agent 先调用 `get_transaction` 读出当前完整记录,并把其余未变更的字段(包括 `budget_id`、`merchant` 与 `comment`)原样带入本次调用,避免遗漏字段被服务端拒绝或静默清空

#### Scenario: 只改分类但漏带商户名
- **WHEN** agent 因为分类判断错误而修正一笔有商户名的交易
- **THEN** agent 在调用前读出原 `merchant` 与 `budget_id`,并在本次调用中原样带上,不用空值覆盖商户名,也不省略预算

### Requirement: 退款冲回原支出的分类与来源
Agent SHALL 在记录一笔退款时,通过先例检索定位被退的原始支出交易,并将退款记入与原支出相同的分类、来源与预算,而不是新建「退款」类目。退款金额的方向按 tally 既有数据模型表示,并在对话预览中呈现。

#### Scenario: 能定位到原始支出
- **WHEN** 用户说某笔支出发生了退款,且 `search_transactions` 能命中对应的原始支出交易
- **THEN** agent 使用该原始交易的 `category_id`、`source_id` 与 `budget_id` 记录退款,并在预览中标明这是对哪笔支出的冲回

#### Scenario: 无法定位原始支出
- **WHEN** 关键词检索无法命中被退的原始支出交易
- **THEN** agent 向用户询问该退款应冲回哪个分类、来源与预算,不擅自新建「退款」分类

### Requirement: 账本数据投影不写入 Agent 记忆
Agent SHALL 将商户→分类/来源/预算这类账本数据的投影关系交给先例检索现算,不写入 Agent 侧持久记忆;只有描述用户本人偏好的信息才归入该记忆。

#### Scenario: 商户到分类的稳定映射
- **WHEN** agent 观察到某个商户/关键词稳定对应某个分类、来源或预算
- **THEN** agent 依赖先例检索现算这个映射,不将其写入 Agent 侧持久记忆

#### Scenario: 用户本人的记账偏好
- **WHEN** agent 了解到用户本人的记账语言、分类粒度偏好等个人习惯
- **THEN** agent 可以将其写入 Agent 侧持久记忆,因为该信息描述的是用户本人而非账本数据
