# plugin-record-skill Specification

## Purpose

定义 agent 在通过 tally MCP 记录一笔新交易或修正一笔已有交易时必须遵守的对话流程与纪律——先例检索、对话预览确认、comment 原文留痕,以及无合适分类时如何安全地顺手新建。

## Requirements

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
Agent SHALL 把用户的自然语言时间表达解析为具体的 Unix 秒数,并在对话预览中展示解析后的日期时间供用户核实。解析 SHALL 通过 shell 的 `date` 命令完成(例如用 `date` 读取当前时刻、用 `date -d "<相对表达>"` 求相对日期、用 `date -d @<ts>` 反查),不得凭心算、训练知识或对话上下文推断日期与星期。所有相对表达(如「今天」「昨天」「前天」「上周五」「3 天前」)SHALL 以运行时注入的当前日期为锚点计算;当缺少注入锚点时,agent SHALL 先运行 `date` 取当前时刻,再据此计算。

#### Scenario: 解析自然语言时间表达
- **WHEN** 用户用自然语言描述交易发生时间(如「今天」「昨天下午三点」「前天」「上周五」)
- **THEN** agent 以注入的当前日期为锚点,用 `date` 命令计算出具体日期时间,按用户本地时区(缺少更明确信息时的默认假设)转为 Unix 秒数,并在对话预览中展示解析出的具体日期、星期与时间

#### Scenario: 缺少注入的时间锚点
- **WHEN** 对话上下文中没有可用的当前日期锚点
- **THEN** agent 先运行 `date` 取当前时刻与本地时区,再据此解析用户的时间表达,不直接凭印象填写

#### Scenario: 绝对日期或时间戳换算
- **WHEN** agent 需要在 Unix 秒数与人类可读日期之间转换(如校验一笔历史交易的日期)
- **THEN** agent 通过 `date` 命令完成换算,不心算

### Requirement: 跨品类消费应提议拆分为多笔
Agent SHALL 在记录一笔涵盖多个商品子类的商户消费(典型场景:超市、综合电商订单)时,于对话预览中提议按商品子类拆分为多笔交易,由用户在预览阶段决定是否拆分以及如何拆分。Agent SHALL NOT 在未经用户确认的情况下,自行把这类消费合并记为单一笼统分类。

#### Scenario: 一张小票涵盖多个品类
- **WHEN** 用户提供的一笔消费明细里含有分属不同分类的商品(如食材、日用、零食混在一次超市结账)
- **THEN** agent 在对话预览中给出按子类拆分的建议方案(每笔的金额、分类、来源、comment),询问用户是否按此拆分或如何调整

#### Scenario: 用户选择不拆分
- **WHEN** 用户在预览阶段表示这笔不需要拆分
- **THEN** agent 按用户指定的单一分类记为一笔,不再坚持拆分

#### Scenario: 拆分后逐笔写入
- **WHEN** 用户确认了拆分方案
- **THEN** agent 为每个子类分别调用 `create_transaction`,每笔各自带上该子类对应的分类、来源与 comment

### Requirement: 退款冲回原支出的分类与来源
Agent SHALL 在记录一笔退款时,通过先例检索定位被退的原始支出交易,并将退款记入与原支出相同的分类与来源,而不是新建「退款」类目。退款金额的方向按 tally 既有数据模型表示,并在对话预览中呈现。

#### Scenario: 能定位到原始支出
- **WHEN** 用户说某笔支出发生了退款,且 `search_transactions` 能命中对应的原始支出交易
- **THEN** agent 使用该原始交易的 `category_id` 与 `source_id` 记录退款,并在预览中标明这是对哪笔支出的冲回

#### Scenario: 无法定位原始支出
- **WHEN** 关键词检索无法命中被退的原始支出交易
- **THEN** agent 向用户询问该退款应冲回哪个分类与来源,不擅自新建「退款」分类

### Requirement: comment 同时承载商户原名与商品明细
Agent SHALL 保证写入的 `comment` 既包含商户原名、也包含足以区分该笔的商品明细,因为 tally 没有独立的商户字段,`search_transactions` 只能在 `comment` 上做子串匹配。当一笔消费被拆分为多笔时,每一笔的 `comment` SHALL 各自带上商户原名与该笔对应的商品明细,使每笔都能被单独检索命中。

#### Scenario: 单笔消费写入 comment
- **WHEN** 记录一笔普通消费
- **THEN** `comment` 中同时出现商户原名与用户表达的商品/事由明细,而非只有其中一项

#### Scenario: 拆分消费的每一笔
- **WHEN** 一笔超市消费被拆分为多笔写入
- **THEN** 每一笔的 `comment` 都以商户原名开头,并附上该笔对应的具体商品明细,任取一笔都能通过商户名或商品名被 `search_transactions` 命中

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
