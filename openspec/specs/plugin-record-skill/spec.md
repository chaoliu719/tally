# plugin-record-skill Specification

## Purpose

定义 agent 在通过 tally MCP 记录一笔新交易或修正一笔已有交易时必须遵守的对话流程与纪律——先例检索、对话预览确认、comment 原文留痕,以及无合适分类时如何安全地顺手新建。

## Requirements

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

### Requirement: 无合适分类时可在预览中提议新建
Agent SHALL 在先例检索与现有分类均无匹配选项时,于对话预览中向用户提议创建新分类,用户确认后再创建并引用。

#### Scenario: 先例检索和已有分类都没有匹配
- **WHEN** 记录一笔新交易时,关键词检索没有先例、且该账本现有分类里没有语义匹配的选项
- **THEN** agent 在对话预览中提议创建一个新分类(说明名称与父分类),用户确认后调用 `manage_category`(`operation="create"`)创建,再引用该新分类完成记录

#### Scenario: 用户拒绝新建分类
- **WHEN** 用户在预览阶段拒绝新建分类的提议
- **THEN** agent 改为询问应该使用哪个已有分类,不擅自创建

### Requirement: 时间字段需要显式解析并在预览中呈现
Agent SHALL 把用户的自然语言时间表达解析为具体的本地日期时间字符串(格式 `YYYY-MM-DD HH:MM:SS`,不带时区、不做任何时区换算——它就是用户记忆里的墙上钟时刻,不因用户当时是否在异地而改变),并在对话预览中展示解析后的日期时间供用户核实。解析 SHALL 通过 shell 的 `date` 命令完成(例如用 `date` 读取当前时刻、用 `date -d "<相对表达>"` 求相对日期),不得凭心算、训练知识或对话上下文推断日期与星期。所有相对表达(如「今天」「昨天」「前天」「上周五」「3 天前」)SHALL 以运行时注入的当前日期为锚点计算;当缺少注入锚点时,agent SHALL 先运行 `date` 取当前时刻,再据此计算。

#### Scenario: 解析自然语言时间表达
- **WHEN** 用户用自然语言描述交易发生时间(如「今天」「昨天下午三点」「前天」「上周五」)
- **THEN** agent 以注入的当前日期为锚点,用 `date` 命令计算出具体日期时间,格式化为 `YYYY-MM-DD HH:MM:SS` 字符串直接作为 `time` 使用,并在对话预览中展示解析出的具体日期、星期与时间

#### Scenario: 缺少注入的时间锚点
- **WHEN** 对话上下文中没有可用的当前日期锚点
- **THEN** agent 先运行 `date` 取当前时刻,再据此解析用户的时间表达,不直接凭印象填写

#### Scenario: 绝对日期或时间戳换算
- **WHEN** agent 需要核对一笔历史交易的发生日期(如判断它是否落在某个相对表达描述的区间内)
- **THEN** agent 直接读取该交易 `time` 字段本身的 `YYYY-MM-DD HH:MM:SS` 字符串进行比较,不再需要与 Unix 秒数互相换算

#### Scenario: 用户在异地记账
- **WHEN** 用户身处与其惯常所在地不同的时区记录一笔交易,并用当地的墙上钟时间描述发生时刻(如"11:00")
- **THEN** agent 原样把这个墙上钟时刻解析为 `time` 字符串,不将其换算成用户惯常时区下的对应时刻,也不询问或推断用户当时所在的时区

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

### Requirement: 退款冲回原支出的分类与来源
Agent SHALL 在记录一笔退款时,通过先例检索定位被退的原始支出交易,并将退款记入与原支出相同的分类、来源与预算,而不是新建「退款」类目。退款金额的方向按 tally 既有数据模型表示,并在对话预览中呈现。

#### Scenario: 能定位到原始支出
- **WHEN** 用户说某笔支出发生了退款,且 `search_transactions` 能命中对应的原始支出交易
- **THEN** agent 使用该原始交易的 `category_id`、`source_id` 与 `budget_id` 记录退款,并在预览中标明这是对哪笔支出的冲回

#### Scenario: 无法定位原始支出
- **WHEN** 关键词检索无法命中被退的原始支出交易
- **THEN** agent 向用户询问该退款应冲回哪个分类、来源与预算,不擅自新建「退款」分类

### Requirement: keyword 检索按字面子串匹配
Agent SHALL 将 `search_transactions` 的 `keyword` 参数中的 `%`、`_` 视为字面字符,不依赖它们实现模糊/通配匹配。

#### Scenario: 使用 % 或 _ 作为关键词的一部分
- **WHEN** agent 构造 `search_transactions` 的 `keyword` 参数
- **THEN** agent 将 `%` 与 `_` 当作字面字符处理,不假设它们会被服务端解释为通配符

### Requirement: 账本数据投影不写入 Agent 记忆
Agent SHALL 将商户→分类/来源/预算这类账本数据的投影关系交给先例检索现算,不写入 Agent 侧持久记忆;只有描述用户本人偏好的信息才归入该记忆。

#### Scenario: 商户到分类的稳定映射
- **WHEN** agent 观察到某个商户/关键词稳定对应某个分类、来源或预算
- **THEN** agent 依赖先例检索现算这个映射,不将其写入 Agent 侧持久记忆

#### Scenario: 用户本人的记账偏好
- **WHEN** agent 了解到用户本人的记账语言、分类粒度偏好等个人习惯
- **THEN** agent 可以将其写入 Agent 侧持久记忆,因为该信息描述的是用户本人而非账本数据

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

### Requirement: 记录前必须为交易选定预算
Agent SHALL 在记录一笔新交易时为其选定一个预算。预算的选择 SHALL 依据交易本身的性质(是否周期性且刚性不可省、是否计划内的大额、是否计划外不得不花,其余归 `日常`),而不是依据它的分类——分类回答"钱花在什么上",回答不了"这笔钱算哪一类"。四个预算的实际含义 SHALL 按账本口径对应:`固定` 是每年、每月必须花的钱(物业费、水电气、话费网费、车位费、车险、社保医保、回老家路费等);`专项` 是大额且计划内的整件事,当前包括电子产品的大额购置与旅游(出游、国外购物),以后可以新增别的专项品类;`突发` 是大额意外与突发医疗等计划外支出;其余日常开销与小额偶然开销归 `日常`,金额小即有明确用途也不归 `专项`。性质无法判断时,Agent SHALL 依次参考同商户、同分类的历史交易落在哪个预算;仍无法确定、历史出现多个预算、或金额明显偏离该商户或分类的历史区间时,Agent SHALL 向用户询问,不得自行猜测或套用默认值。

#### Scenario: 性质明确的交易
- **WHEN** 一笔交易的性质可以从商户、备注或金额直接判断(如年度车位租赁费、话费、突发的医疗支出)
- **THEN** Agent 据此选定预算,不因该交易的分类而改判

#### Scenario: 小额但用途专一的支出
- **WHEN** 一笔金额很小的支出有明确用途(如为某次出游添置的泳衣、为特定场合买的衣服)
- **THEN** Agent 将其归入 `日常`,不因用途专一而归 `专项`

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
