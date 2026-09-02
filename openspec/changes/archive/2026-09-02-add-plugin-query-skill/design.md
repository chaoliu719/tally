## Context

见 proposal.md — Why。三个已实现 skill 的触发面之间的缝隙在实际使用中反复暴露为
「查账时间全错」。本 change 把「只读查账」确立为第四种任务形状,并把时间换算纪律
在查询的**输入侧和输出侧**都写成硬约束。

## Goals / Non-Goals

**Goals:**
- 纯查询意图有一个明确、会触发的 skill 承接。
- 查询前(窗口 → 秒数)、查询后(秒数 → 人类时间)两次换算都强制走 shell `date`。
- `query` 与 `analysis` / `record` / `optimize` 的边界在四个 skill 正文里互相自洽。

**Non-Goals:**
- 不做汇总、环比、趋势——那是 `analysis`,`query` 命中「要合计」时转过去。
- 不触碰 `mcp/`;`time` 仍是无时区语义的 Unix 秒。
- 不新增 MCP 工具;`query` 只组合现有的 `search_transactions` / `get_transaction` /
  `list_ledgers`。
- 不用 hook 强制时间换算(语义判断留给 skill/对话,与 `plugin-time-context-hook`
  的既有立场一致)。

## Decisions

### D1: 新建独立 skill,而不是扩 `analysis` 的 description

- 备选:把 `analysis` 的 description 放宽到覆盖所有读操作 —— 被否。skill 名与定位会
  变糊,轻量的「列一下昨天的账」也要拉进整份分析 skill;而且 `analysis` 正文全是
  「拼多次查询做对比」,对纯查询是噪声。
- 四个 skill 各对应一种任务形状(记 / 查 / 分析 / 整理结构)边界最清晰。

### D2: 时间换算纪律在 `query` 里重述,不做成共享片段被引用

- skill 是独立加载的:用户可能只触发 `query` 而不触发 `record`/`analysis`。换算纪律
  必须在每个碰时间的 skill 正文里自包含。
- `record` 和 `analysis` 已各自有一份措辞贴合自身场景的版本;`query` 再写一份聚焦
  「查询前窗口 + 查询后返回值」两步,可接受的有意重复。

### D3: `analysis` 的 description 收窄写进本 change,但不作为 spec delta

- `description` 是 skill 触发文案,`plugin-analysis-skill` spec 的 Requirement 不描述
  它。收窄只是为了让 `query` 和 `analysis` 的触发不打架,属实现细节,记在 proposal
  的 Impact 里即可,不产生 `MODIFIED` Requirement。

### D4: `query` 只到「取出来、读对、显示对」为止

- 不在 `query` 里维护「什么商户归什么类」之类的规则表(与先例检索冲突、会随分类树
  漂移),这类判断该发生在 `record`。
- 查到的结果如何进一步加工(预算建议、异常点评)归 Agent 自己,不进 skill。

## Risks / Trade-offs

- **[SKILL.md 与 spec 漂移]** 换算纪律与返回值解读同时存在于 spec 和 `SKILL.md` →
  tasks 里把「逐条对照 spec 核对正文」列为独立可勾选项,archive 时复核。
- **[`query` / `analysis` 触发重叠]** 「这个月花了多少」既像查询又像分析 → 两份
  description 都写明判据(要「明细清单」用 query,要「合计/对比」用 analysis),
  正文各自「不在范围内」一节互相指路。
- **[换算仍靠 agent 执行]** skill 只能要求走 `date`,不能强制 → 与 record 同款风险,
  靠 `plugin-time-context-hook` 的锚点 + skill 的明令兜底,不追求 100%。
