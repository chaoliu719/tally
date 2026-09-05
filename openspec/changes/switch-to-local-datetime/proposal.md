## Why

`time` 目前是无时区语义的 Unix 秒数,靠 agent 在会话里用 shell `date` 猜用户本地时区换算写入、猜相反方向换算读出。这个模型假设"记账时的本地时区"和"任何后续查看时刻"是同一个时区,但用户跨时区旅行时并不成立:在韩国 11:00 记的一笔账,回国后仍然是用户记忆里的"11:00"这个墙上钟时刻,不会因为物理位置变化而追认成 12:00。当前模型还带来一个更隐蔽的正确性问题——写入时靠 agent 猜时区、`open_transaction_timeline` 的退化摘要硬编码 UTC+8、widget 客户端按查看者浏览器本地时区渲染,三处时区假设互相独立、可能互不一致,导致同一笔交易的"哪一天"在不同环节读出不同答案。

改成存储用户口述的本地日期时间(naive,无时区标记,不做任何时区换算),使 `time` 字段的语义从"世界统一时间轴上的一个瞬时点"变成"用户记忆里的墙上钟时刻",从根本上消除这三处时区假设互相打架的可能,同时去掉 agent 侧计算 Unix 时区偏移的整类工作。

不考虑历史数据迁移——現有 `transactions.time` 列的既存数据可以直接丢弃重来。

顺带解决一个同属"时间锚点"范畴、但成因不同的独立问题:`plugin/hooks/time-context.sh` 目前规定锚点必须"来自运行时的系统时钟",这个假设在 Claude Code 本地会话里成立(系统时钟就是用户所在地的时钟),但在 cowork / claude.ai 网页版这类托管执行环境里不成立——hook 实际跑在 Anthropic 的服务器上,系统时区是 UTC,不是用户实际所在地的 CST+0800,会让"今天"在 UTC 16:00–24:00(对应用户本地 0:00–8:00)这段时间被算错一天。但没必要给 hook 打补丁修这个 bug——Claude Code / claude.ai 的系统提示词本身已经会注入当前日期(`currentDate`),这份注入是宿主平台级的能力,不依赖 hook 执行所在主机的系统时钟,天然不受"hook 跑在哪台机器上"这个问题影响。tally 自己的 `time-context` hook 只是在重复宿主已经提供、而且提供得更可靠的信息,直接删除即可,不需要另外维护一份自己的时区判定逻辑。

## What Changes

- **BREAKING**:`transactions` 表的 `time` 列类型从 `INTEGER`(unix seconds)改为 `TEXT`,格式固定为 `YYYY-MM-DD HH:MM:SS`(24 小时制、零填充、naive,不带任何时区标记)。因为该格式的字符串字典序与时间序等价,现有 `ORDER BY time, id` 及范围过滤(`start_time`/`end_time`/`after_time` 的 `>=`/`<=`/`>`/`<` 比较)在 SQL 层无需改变比较逻辑,只改列类型。
- **BREAKING**:`create_transaction`/`update_transaction` 的 `time` 入参,以及 `search_transactions`/`get_financial_summary` 的 `start_time`/`end_time` 入参,`get_transaction`/`search_transactions`/`update_transaction` 等工具返回值里的 `time`,全部从"unix seconds 整数"改为"`YYYY-MM-DD HH:MM:SS` 字符串",格式非法时报校验错误(替换原先的 `txTime <= 0` 校验)。
- 搜索/时间线分页游标(`encodeSearchTransactionsCursor`/`decodeSearchTransactionsCursor`)编码的时间部分从 `int64` 改为字符串,游标本身仍不透明,不影响外部行为。
- `open_transaction_timeline` 的退化摘要(`timeline.go` 里的 `summaryZone`/`formatUnixDate`)删除硬编码的 `UTC+8` 时区换算,改为直接格式化已存储的本地日期时间字符串。
- Timeline widget(`timeline.html`)删除所有 `new Date(unixSeconds * 1000)` + `Intl.DateTimeFormat` 的时区转换逻辑,改为直接解析/分组/展示存储的日期时间字符串——不再依赖查看者浏览器所在时区。
- `plugin/skills/{record,query,analysis}/SKILL.md` 里"把用户说的时间转成 Unix 秒 / 反查时用 `date -d @<ts>` 换算"的段落改写:相对日期解析(依赖 `time-context` hook 注入的锚点)不变,但落地到 `time`/`start_time`/`end_time` 的最终产物从 epoch 秒改为直接传日期时间字符串,反查显示也不再需要时区换算。
- 更新 `openspec/config.yaml` 项目 context 中"不做任何时区处理——时间字段一直是纯 Unix 秒数"这句过时表述(在 design.md 中记录本次变更对该决策的推翻)。
- **BREAKING**(移除):删除 `plugin/hooks/time-context.sh` 及其在 `plugin/hooks/hooks.json` 里的 `SessionStart`/`UserPromptSubmit` 注册。当前日期锚点改为直接依赖宿主(Claude Code / claude.ai)系统提示词自带的 `currentDate` 注入,不再由 tally 自己的 hook 重复计算或注入"本地时区"。三份 SKILL.md 里提到"tally plugin 的 time-context hook"的措辞改为指向宿主自带的当前日期上下文。

## Capabilities

### New Capabilities

（无——本变更不引入新能力,只改变既有能力对 `time` 字段的表述与格式。）

### Modified Capabilities

- `transaction-recording`:`create_transaction`/`update_transaction`/`search_transactions`/`get_transaction` 中 `time`/`start_time`/`end_time`/游标的字段格式与校验规则,从 Unix 秒改为本地日期时间字符串。
- `financial-analytics`:`get_financial_summary` 的 `start_time`/`end_time` 入参格式同上。
- `transaction-timeline-widget`:`open_transaction_timeline` 退化摘要与 widget 渲染不再做任何时区换算,直接使用存储的本地日期时间字符串。
- `plugin-record-skill`:时间解析纪律改写为"解析出日期时间字符串直接写入",不再要求心算/换算 Unix 秒。
- `plugin-query-skill`:时间范围解析与结果展示纪律同上,不再需要 `date -r`/`date -d @<ts>` 反查换算;「查询前把时间窗口换算」这条 Requirement 也不再引用 `plugin-time-context-hook`,改为引用宿主系统提示词自带的当前日期。
- `plugin-analysis-skill`:时间范围解析纪律同上。

### Removed Capabilities

- `plugin-time-context-hook`:整个能力(hook 脚本、`hooks.json` 注册、四条 Requirement)全部移除。当前日期锚点改由宿主(Claude Code / claude.ai)系统提示词自带的 `currentDate` 注入提供,tally 不再需要自己的 hook 重复这件事。

## Impact

- **DB**:`mcp/internal/store/schema.sql`(列类型)、`queries.sql`(注释与生成代码类型)。无迁移脚本,存量数据库直接重建。
- **Go 工具层**:`mcp/internal/tools/transactions.go`、`transactions_cursor.go`、`analytics.go`、`timeline.go`。
- **Widget**:`mcp/internal/widgets/timeline.html`。
- **Skill**:`plugin/skills/record/SKILL.md`、`plugin/skills/query/SKILL.md`、`plugin/skills/analysis/SKILL.md`。
- **Hook**:删除 `plugin/hooks/time-context.sh` 与 `plugin/hooks/hooks.json` 里对应的 `SessionStart`/`UserPromptSubmit` 注册;`plugin/skills/{record,query,analysis}/SKILL.md` 里引用该 hook 的措辞改为引用宿主系统提示词自带的当前日期。
- **不受影响**:`oauth/token.go`、`oauth/server.go`、`confirm/confirm.go` 里的时间戳是 OAuth token 过期、写操作确认令牌有效期,属于真正的绝对瞬时点语义,不在本变更范围内。
