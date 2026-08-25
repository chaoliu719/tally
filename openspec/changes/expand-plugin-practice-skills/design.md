## Context

`plugin/skills/record/` 已经实现但从未被 speced;这次连带把它现有行为(先例检索、
对话预览、comment 纪律)首次写成正式 spec,再叠加一条新行为(无合适分类时提议新建)。
`plugin/skills/analysis/` 与 `plugin/skills/optimize/` 是全新 skill。三者都只消费
`mcp/` 已有的工具(`search_transactions`、`get_financial_summary`、`manage_category`、
`update_transaction`、`get_transaction`),不新增任何 MCP 工具或 mcp 侧行为。见
proposal.md 的 Why/What Changes。

## Goals / Non-Goals

**Goals:**
- 把已经讨论定型的三块内容(record 补分类新建、analysis 的查询与解读规则、optimize
  的拆分/合并检测与安全执行流程)落成可归档的 spec 与 skill 正文。
- optimize 里"提议周期性复盘"这部分,要不依赖任何具体 harness 的定时 API 就能工作。

**Non-Goals:**
- 不做可视化/图表输出(analysis 明确只产出文字/表格)。
- 不新增批量更新类 MCP 工具——合并分类时的逐笔重新分类,仍然是多次
  `update_transaction` 调用,这是本次改动接受的限制,不是要解决的问题。
- 不做 manage 类操作的独立 skill——ledger/source/category 的增删改仍然主要靠工具
  自身的 schema 描述与报错自解释,本次只在 record(顺手建分类)与 optimize(拆分/
  合并分类)两个有具体业务动机的地方触碰 `manage_category`。

## Decisions

**1. optimize 的定时复盘提议不硬编码具体调度机制,而是让 agent 检查当次会话暴露的
工具/skill 列表。**

原因:定时能力(Claude Code 的 `schedule` skill/`scheduled-tasks` MCP 工具、或
`CronCreate` 一类底层工具)是 harness 层的能力,不是 MCP 协议或 plugin 打包格式保证
存在的东西——不同 Claude Code 环境是否连了这层能力都可能不同,更不用说这份 skill
理论上可能被拿到别的 MCP host 里用。如果 skill 正文里写死"调用 XX 工具",一旦
harness 把这个工具改名或者当前环境根本没连这个能力,skill 就会给出错误指令。

相反,任何支持定时的 harness,原理上都会像现在这次会话一样,把对应的工具或 skill
自动列进模型自己的系统提示词/工具箱——所以 skill 只需要教"什么时候该提议、提议
内容是什么",怎么真正调度交给 agent 自己观察当次可用的能力去决定。这条决策是本次
change 里唯一需要 design.md 记录的技术选择,因为它直接决定了 optimize spec 里
"周期性复盘建议不硬编码具体调度机制"这条需求怎么写、怎么验证。

备选方案(未采用):让 skill 直接指名调用某个具体工具(如 `CronCreate`)。放弃原因
如上——牺牲可移植性换来的确定性在这个场景不值得,而且底层工具名本身也不是这次
讨论里能百分百确认的稳定 API。

**2. record 首次补 spec,一并覆盖已实现的旧行为和本次新加的行为,而不是只写新增的
delta。**

原因:`plugin-record-skill` 目前没有任何既有 spec,如果只 spec"顺手建分类"这一条
新行为,读者看这份 spec 会看不到先例检索/对话预览/comment 纪律这些已经在生产的核心
流程,capability spec 会显得残缺。所以把已实现的行为也写成 `ADDED Requirements`——
这是这个 capability 的第一份 spec,不是在一个已有 spec 上打 delta。

**3. 分类合并通过逐笔 `update_transaction` + 空分类走 `manage_category` delete 完成,
不等待未来的批量更新工具。**

原因:`mcp/` 侧目前没有批量更新交易的工具,config.yaml 里"批量导入/导出"明确是延后项
且现在不提案。个人记账规模下,单个分类下的交易量通常是几十笔量级,逐笔调用可接受;
真正量大的情况通过"交易量较大时提前告知成本"这条需求兜底,让用户自己判断是否值得
现在做,而不是在 mcp 侧为这一个场景现在就造一个批量工具。

## Risks / Trade-offs

- [分类下交易数量很大时,合并操作需要很多次 `update_transaction` 调用,耗时且消耗
  对话上下文] → 缓解:合并前 optimize skill 必须先展示受影响交易数量,由用户决定
  是否现在执行;真正需要批量能力时,应该走 config.yaml 里已经定型的
  `batch_create_transactions`/批量方向的独立提案,而不是让 optimize 自己造轮子。
- [周期性复盘提议依赖"当次会话是否暴露了定时能力"这一运行时判断,不同环境行为不
  一致] → 缓解:这是本次刻意接受的设计,以可移植性换确定性;skill 里要求 agent 在
  没有能力时明确告知用户,而不是假装设置成功。
- [三份 skill 正文本身不受自动化测试覆盖,spec 里的 Scenario 更多是行为契约而非
  可执行测试] → 缓解:沿用 record 已经建立的模式——behavior 是否被遵守,靠人工
  在真实对话里验证(仓库其他 skill 也是这个验证方式),不属于本次要新增的机制。
