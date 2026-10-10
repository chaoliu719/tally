## Context

见 proposal.md - Why。补充现状事实:

- `OpenTransactionTimelineInput` 只有 `ledger_id`(`mcp/internal/tools/timeline.go:40`)。首屏查询 `SearchTransactionsDesc` 取 200 条,游标指纹 `searchTransactionsFilterFields` 里只有 `LedgerID` 与 `NewestFirst`。
- 工具结果的文本内容是 `summary + "\n\n" + JSON(payload)`(`timeline.go:150`),所以支持 widget 的宿主里 Agent 也会读到 200 条明细。面板侧 `extractFirstPagePayload` 在"结果被 text-only 投递"时能从文本块里解析 JSON(见 archive/timeline-widget-local-filtering 的风险项),所以去掉文本里的 JSON 之前必须验证这条路径不被依赖。
- `search_transactions` 已支持 `source_id`、`category_id` + `include_descendants`、`start_time` / `end_time`;分类子孙展开用 `deps.Q.ListCategoryDescendantIDs`,再以 `category_ids_json` 传入查询(`transactions.go:702-717`)。**游标内嵌了过滤条件的指纹,翻页必须带与首页完全一致的过滤参数**,这是后台取数能"只在范围内"的关键约束。
- 面板的日期筛选、分类/来源筛选、顶部统计都是本地的(archive/timeline-widget-local-filtering),顶部统计目前用结果里的 `total`(账本总数,来自 `TransactionStats`)。
- `get_financial_summary` 已有"按币种汇总收入/支出 + `currency.FormatMajor` 格式化"的成熟做法(`analytics.go`、`SummarizeTransactionsByCurrency`),金额库里是最小单位整数,收入为正、支出为负。
- 每次 `callServerTool` 桥接往返固定 2-5s,与数据量无关;所以"少拉数据"的真正收益来自把范围放在首屏就一次给齐、并让后台取数只在范围内进行(轮数变少)。
- plugin 侧已有 `plugin/skills/query/SKILL.md`(查账 skill,含 `date` 换算纪律),目前没有提到时间线工具。

## Goals / Non-Goals

**Goals:**
- Agent 打开时间线时能精确划定"哪个账本、哪段时间、哪个分类/来源",面板只加载这一范围。
- 工具本身语义干净:参数可选、无隐含默认;"默认看最近 30 天"这类使用习惯只存在于 skill。
- 面板始终让用户看得出当前范围,避免"以为看的是全部,实际是预置范围"的误解。
- 工具结果不再把交易明细写进 Agent 的对话上下文。

**Non-Goals:**
- 不支持多分类、多来源(单值,与 `search_transactions` 一致)。
- 不在面板里扩大范围或切换范围;换范围一律由 Agent 重新打开。
- 不换算币种,不做跨币种合计。
- 不改 `search_transactions` 的参数与游标协议,不改后台取数"分批 + 游标"的机制,不做虚拟滚动等大数据量优化。
- 不改变筛选条的本地筛选机制本身(只在已加载范围内收窄)。

## Decisions

### D1: 工具参数与日期语义

`OpenTransactionTimelineInput` 新增:

```go
StartDate  string `json:"start_date,omitempty"`  // YYYY-MM-DD,含当天
EndDate    string `json:"end_date,omitempty"`    // YYYY-MM-DD,含当天
CategoryID string `json:"category_id,omitempty"` // 单值,含全部子孙分类
SourceID   string `json:"source_id,omitempty"`   // 单值
```

- 日期用严格的 `time.Parse("2006-01-02", ...)` 校验(拒绝 `2026-9-1`、`2026-02-30`);只用于校验与格式,不做任何时区解释。
- 服务端换算为查询用的 `start_time = start_date + " 00:00:00"`、`end_time = end_date + " 23:59:59"`,与库里定宽的 `time` 字符串直接字典序比较(与仓库既有"定宽字符串上直接比较"纪律一致)。
- `start_date > end_date` → 报错,不自动对调。
- **只传 `start_date` 时 `end_date` 不设上界,而不是取"今天"。** 讨论时的口径是"没传就默认今天",但服务端取"今天"必须读服务端时钟并选一个时区,与项目"不做任何时区处理"的原则冲突;而不设上界对任何非未来日期的交易与"到今天"完全等价,只有"晚于今天的未来记账"这一边角会被多包含进来。这是对讨论口径的一处有意偏离,若需要严格"到今天"应由 skill 层(Agent 用 `date` 算好今天)显式传 `end_date`。
- 只传 `end_date` 时不设下界(从账本最早一天起)。
- `category_id` / `source_id`:用 `GetCategory(id, ledger_id)` / `GetSource(id, ledger_id)` 校验属于该账本,不存在则报错。分类始终展开子孙(固定语义,不暴露 `include_descendants` 参数,因为这里没有"只要本级"的使用场景;面板的预置标签也据此写"含子分类")。

**备选**:参数用 `start_time` / `end_time`(`YYYY-MM-DD HH:MM:SS`,与 `search_transactions` 对齐)。否决:浏览面板的粒度是"天",日期更贴近"本月/9 月"的说法,也省去 Agent 拼时分秒的出错机会。
**备选**:提供 `recent_days` 之类简写。否决:Agent 本来就要换算日期,多一个参数只增加歧义。
**备选**:工具默认最近 30 天。否决(用户明确要求):工具不应有默认行为,默认归 skill,这样 Agent 忘了遵守 skill 时退化成"看全部"这一可预测行为,而不是悄悄少了数据。

### D2: 范围在服务端一次性固定,游标指纹带上范围

`openTransactionTimeline` 把四个范围条件构造成 `searchTransactionsFilterFields`(`StartTime` / `EndTime` / `CategoryID` / `IncludeDescendants` / `SourceID`,与 `searchTransactions` 里组装指纹的方式完全一致),用于首屏游标;首屏查询用 `SearchTransactionsDesc` 并带同样的 `source_id` / `category_ids_json` / `start_time` / `end_time`。

结果新增 `scope`:

```go
type TimelineScope struct {
    StartDate, EndDate string // 回显调用方传入的日期(空表示未设)
    StartTime, EndTime string // 实际用于查询的定宽字符串(空表示未设)
    CategoryID, SourceID string
    IncludeDescendants bool   // 有 category_id 时恒为 true
}
```

范围内交易总条数放在结果原有的 `total` 字段里(语义由"账本总数"改为"范围内条数",不传范围时二者相同)。

面板后台取数时,直接用 `scope.StartTime` / `EndTime` / `CategoryID` / `SourceID` / `IncludeDescendants` 作为每页 `search_transactions` 的参数,**不自己重新拼接日期**,从而保证与游标指纹逐字一致,也避免面板与服务端各算一遍日期边界。

### D3: 范围统计:一条新查询,同时给条数与按币种的收支合计

新增 sqlc 查询 `SummarizeTimelineScope`,过滤条件与 `SearchTransactionsDesc` 的范围部分一致(`ledger_id` / `source_id` / `category_ids_json` / `start_time` / `end_time`),按 `currency` 分组返回 `COUNT(*)`、收入合计、支出合计(口径与 `SummarizeTransactionsByCurrency` 相同:收入 `SUM(amount)`、支出 `SUM(-amount)`,CAST 为整数最小单位)。范围内总条数 = 各币种分组 `COUNT` 之和,一次查询同时得出条数与合计,不再需要 `TransactionStats`(账本总数不再展示)。金额用 `currency.FormatMajor` 格式化,与 `get_financial_summary` 保持一致。

**备选**:复用 `SummarizeTransactionsByCurrency`。否决:它没有来源/分类过滤,也不返回条数。
**备选**:在 Go 里对内存中的交易求和。否决:首屏只有一页,合计必须覆盖范围内全量,"在全量数据上用 DB 的速度算"正是项目划给服务端的那类计算。

### D4: 文本摘要只放摘要,不放明细

文本内容改为:生效范围的可读描述、范围内条数、按币种的收入合计与支出合计,并提示"需要明细可用带同样范围的 `search_transactions`"。**不再拼接 JSON 载荷**;交易、`scope`、查找表只走结构化结果(`OpenTransactionTimelineOutput`)。范围内无交易时摘要说明"该范围内没有交易"并带上范围。

**验证结论(任务 1.1)**:`runtime.js` 的 `extractFirstPagePayload` 先读 `structuredContent`(含 `transactions` 数组或数值型 `total` 即采用),仅在结构化结果缺失时才回退解析文本块;用 node 直接加载 `runtime.js`,喂入"只有结构化结果、文本不含 JSON"的结果,首屏载荷读取正常。因此去掉文本里的 JSON 不影响面板。唯一残余风险是"只投递文本、不投递 `structuredContent`"的宿主,MCP Apps 规范下宿主应下发完整的工具结果,本地无法对真实宿主复验,需在部署后于 Claude app 里实际打开一次确认。另外,结果里的 `total` 字段保留,语义改为范围内条数(与 `scope` 一致,旧面板读它也不会出错),因此 `scope` 里不再单独放 `Count`。

**验证门槛(实现第一步)**:先确认面板在目标宿主上通过结构化结果拿到数据,不依赖文本块里的 JSON。若验证发现存在只投递文本的宿主,则本决定降级为"文本里仍保留 JSON 但不含在摘要段",并把该事实记入 spec 的降级说明后再继续——这是唯一会改变任务拆分的未知项,故放在 tasks 第 1 组且先于其余改动。

### D5: 面板的范围展示、只读标签与日期约束

- 顶部:用 `scope` 渲染范围描述(如"2026-09-01 至 2026-09-30 · 餐饮(含子分类) · 共 N 笔");有活跃筛选时条数改为本地匹配数,范围描述不变。顶部不再使用账本总数 `total`;结果缺 `scope` 时(旧服务端)按"全部"处理、不显示范围。
- 预置的分类/来源:对应的 `<select>` 隐藏,在筛选条旁显示只读标签(分类标签附"含子分类")。未预置的保持现有下拉与本地筛选。`filterReset` 只清用户设置的输入,不动预置范围。
- 日期输入:`filterStart` / `filterEnd` 的 `min` / `max` 取自 `scope.StartDate` / `EndDate`(某端未设则不加该属性),使超出范围的日期无法选择;范围某端未设时该端不限。本地比较仍在 `time` 的 `YYYY-MM-DD` 前缀上做字典序比较。
- 空状态文案三分:账本本身为空(`这个账本还没有交易`)、范围内为空(`该范围内没有交易` + 范围)、筛选后为空(`没有符合当前条件的交易`)。

### D6: skill 承载默认行为

在 `plugin/skills/query/SKILL.md` 增加"用时间线浏览"一节(对应 `plugin-query-skill` delta):使用时机(想翻看而非念清单)、无范围时默认最近 30 天并告知用户、`date` 换算(沿用现有的"算一次再传"纪律,只取 `YYYY-MM-DD`)、分类/来源先查 id、换范围就重新打开。skill 的 `description` 触发词也补上"打开时间线 / 翻翻账本"一类说法,使其会被读到。

## Risks / Trade-offs

- **[Agent 没遵守 skill、没传时间范围]** → 工具回到"看全部",与此前行为一致、可预测;面板顶部显示范围(此时无时间限定),用户一眼能看出。
- **[文本去掉明细后,Agent 无法就面板里的数据回答追问]** → 摘要里的条数与按币种合计已覆盖常见追问;需要明细时 Agent 用带同样范围的 `search_transactions`(摘要里给出提示)。这是有意的取舍:浏览数据不进对话上下文。
- **[面板依赖文本块 JSON 的宿主]** → D4 的验证门槛;降级方案已写明。
- **[游标指纹不一致导致后台取数被拒]** → D2 让面板直接复用服务端回传的 `scope` 字段取数,并在 e2e 测试里覆盖"带范围的多页取数"。
- **[范围很大(如全年)时首屏更慢]** → 机制保留(用户已确认),只是仍然只在范围内工作;不设范围上限。
- **[`end_date` 不传时不取"今天"]** → 见 D1;对绝大多数账本等价,未来日期交易会被多包含。
- **[旧面板对接新服务端 / 新面板对接旧服务端]** → 新字段与参数均为增量;新面板缺 `scope` 时降级为"全部"。

## Migration Plan

1. 服务端:新增参数、校验、统计查询、结果字段、文本摘要 → sqlc 重新生成 → 构建镜像 → 按既有部署流程发布并 `curl /healthz`。
2. widget HTML 内嵌在二进制里,随同一镜像发布;宿主缓存 TTL 为 5 分钟,新资源数分钟内自然生效。
3. skill 随 plugin 发布(`plugin/skills/query/SKILL.md`)。
4. 回滚:参数与字段均为可选增量,回退镜像即可,无数据迁移;不传参数的调用行为不变。

## Open Questions

- 摘要里"生效范围"的具体措辞(如何描述只设了一端的范围)可在实现时定,不影响 spec 与任务拆分。
