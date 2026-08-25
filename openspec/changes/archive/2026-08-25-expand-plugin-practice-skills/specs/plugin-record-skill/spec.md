## Purpose

定义 agent 在通过 tally MCP 记录一笔新交易或修正一笔已有交易时必须遵守的对话流程与纪律——先例检索、对话预览确认、comment 原文留痕,以及无合适分类时如何安全地顺手新建。

## ADDED Requirements

### Requirement: 记录前必须检索先例
Agent SHALL 在为一笔新交易选择来源与分类之前,先用 `search_transactions` 按商户/关键词检索该账本内 `comment` 命中的历史交易,并把命中先例的 `source_id`/`category_id` 作为默认选择。

#### Scenario: 存在同关键词历史交易
- **WHEN** 该账本内存在 `comment` 命中同商户/关键词的历史交易
- **THEN** agent 使用这些历史交易的 `source_id`/`category_id` 作为默认选择,不要求用户重新提供

#### Scenario: 未搜到先例
- **WHEN** 关键词检索没有命中任何历史交易
- **THEN** agent 向用户询问来源与分类,而不是凭空猜测

### Requirement: 写入前必须给出对话预览
Agent SHALL 在调用 `create_transaction` 或 `update_transaction` 之前,先在对话中给出包含全部字段的结构化预览,等待用户确认或修改。

#### Scenario: 预览包含完整字段
- **WHEN** agent 已经确定好一笔新交易的金额、来源、分类、时间与 `comment`,准备写入
- **THEN** agent 先展示这些字段的结构化预览,用户确认后才调用 `create_transaction`

#### Scenario: 用户要求修改预览
- **WHEN** 用户在预览阶段要求修改某个字段
- **THEN** agent 更新预览内容并重新等待确认,不跳过确认直接写入

### Requirement: comment 必须保留原始凭据文本
Agent SHALL 保证 `comment` 字段记录的是用户原始表达或账单/商户原名,而不是 agent 自己归类后的解释性文字;修正一笔已有交易时,除非用户明确要求修改原文本身,`comment` SHALL 保持不变。

#### Scenario: 新建交易写入 comment
- **WHEN** 记录一笔新交易
- **THEN** `comment` 字段是用户表达的原始文本或账单/商户原名,不是 agent 归类后的解释

#### Scenario: 修正交易时保留 comment
- **WHEN** 因为分类/来源/金额判断错误而调用 `update_transaction` 修正一笔已有交易,且用户没有明确要求修改 `comment` 本身
- **THEN** agent 保持 `comment` 与修正前一致,不省略也不改写该字段

### Requirement: 修正交易前必须读出当前完整记录
Agent SHALL 在调用 `update_transaction` 之前,先用 `get_transaction` 获取该交易当前的完整字段值,并在 `update_transaction` 调用中带上全部未变更字段的原值。

#### Scenario: 调用 update_transaction 前
- **WHEN** agent 准备调用 `update_transaction` 修正一笔交易的任意字段
- **THEN** agent 先调用 `get_transaction` 读出当前完整记录,并把其余未变更的字段(包括 `comment`)原样带入本次调用,避免遗漏字段被服务端静默清空

### Requirement: 无合适分类时可在预览中提议新建
Agent SHALL 在先例检索与现有分类均无匹配选项时,于对话预览中向用户提议创建新分类,用户确认后再创建并引用。

#### Scenario: 先例检索和已有分类都没有匹配
- **WHEN** 记录一笔新交易时,关键词检索没有先例、且该账本现有分类里没有语义匹配的选项
- **THEN** agent 在对话预览中提议创建一个新分类(说明名称与父分类),用户确认后调用 `manage_category`(`operation="create"`)创建,再引用该新分类完成记录

#### Scenario: 用户拒绝新建分类
- **WHEN** 用户在预览阶段拒绝新建分类的提议
- **THEN** agent 改为询问应该使用哪个已有分类,不擅自创建

### Requirement: 时间字段需要显式解析并在预览中呈现
Agent SHALL 把用户的自然语言时间表达解析为具体的 Unix 秒数,并在对话预览中展示解析后的日期时间供用户核实。

#### Scenario: 解析自然语言时间表达
- **WHEN** 用户用自然语言描述交易发生时间(如"今天""昨天下午三点")
- **THEN** agent 按用户本地时区(缺少更明确信息时的默认假设)解析为 Unix 秒数,并在对话预览中展示解析出的具体日期时间

### Requirement: keyword 检索按字面子串匹配
Agent SHALL 将 `search_transactions` 的 `keyword` 参数中的 `%`、`_` 视为字面字符,不依赖它们实现模糊/通配匹配。

#### Scenario: 使用 % 或 _ 作为关键词的一部分
- **WHEN** agent 构造 `search_transactions` 的 `keyword` 参数
- **THEN** agent 将 `%` 与 `_` 当作字面字符处理,不假设它们会被服务端解释为通配符

### Requirement: 账本数据投影不写入 Agent 记忆
Agent SHALL 将商户→分类/来源这类账本数据的投影关系交给先例检索现算,不写入 Agent 侧持久记忆;只有描述用户本人偏好的信息才归入该记忆。

#### Scenario: 商户到分类的稳定映射
- **WHEN** agent 观察到某个商户/关键词稳定对应某个分类或来源
- **THEN** agent 依赖先例检索现算这个映射,不将其写入 Agent 侧持久记忆

#### Scenario: 用户本人的记账偏好
- **WHEN** agent 了解到用户本人的记账语言、分类粒度偏好等个人习惯
- **THEN** agent 可以将其写入 Agent 侧持久记忆,因为该信息描述的是用户本人而非账本数据
