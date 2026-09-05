## Context

见 proposal.md - Why。现状:`transactions.time` 是 `INTEGER`(unix seconds),Go 侧 `int64`,三处独立的时区假设互相不知道对方存在——写入时 agent 用 shell `date` 猜用户本地时区换算成 epoch;`mcp/internal/tools/timeline.go` 的退化摘要硬编码 `time.FixedZone("UTC+8", ...)`;`mcp/internal/widgets/timeline.html` 用 `new Date(unixSeconds*1000)` + `Intl.DateTimeFormat(undefined, ...)` 按查看者浏览器本地时区渲染。`plugin/config.yaml` 项目 context 与 `openspec/changes/archive/2026-09-01-harden-record-skill-add-time-hook/design.md` 都明确记录过"不做时区处理,`time` 永远是无时区语义的 Unix 秒数"这条决策——本 change 是对它的显式推翻,原因见 proposal.md - Why(用户跨时区旅行时,记忆中的墙上钟时刻不随物理位置回溯改变)。

不考虑历史数据迁移:`schema.sql` 注释本就写明"没有迁移框架,只有一个 schema 版本,靠 `CREATE TABLE IF NOT EXISTS` 一次性建好";生产库可直接删除重建(用户已确认存量数据可以丢弃)。

## Goals / Non-Goals

**Goals:**
- `time`/`start_time`/`end_time`/游标里的时间部分,从"Unix 秒整数"改为"`YYYY-MM-DD HH:MM:SS` 本地日期时间字符串",协议层不做任何时区解释、存储、比较、排序均按字符串原样处理。
- 删除现存的四处独立时区假设(agent 侧换算、`timeline.go` 硬编码 UTC+8、widget 客户端按浏览器时区渲染、`time-context` hook 依赖执行主机系统时钟),不引入新的时区假设替代它们——包括不新增任何 tally 自己维护的时区常量或判定逻辑。
- 保持现有排序/分页/范围过滤的 SQL 逻辑不变(字符串字典序与时间序等价,`ORDER BY`/`>=`/`<=` 语义不变)。

**Non-Goals:**
- 不新增时区/地区列,不做任何时区探测、地理定位或多时区支持——这正是要去掉的复杂度,不是要更精细地做时区。
- 不做历史数据迁移或双写兼容;不保留旧的 Unix 秒格式作为向后兼容的次要格式。
- 不改变 `search_transactions`/`get_financial_summary` 现有的排序、分页、筛选组合逻辑本身,只改时间值的表示格式。
- 不触碰 `oauth`/`confirm` 包里 `time.Now()` 产生的、代表真实瞬时点的时间戳(token 过期、确认令牌有效期)——那些字段的语义没有变化。

## Decisions

### D1: 存储格式选 `TEXT`,固定 `YYYY-MM-DD HH:MM:SS`,不选 `INTEGER` 编码或新增时区列

- 选定:SQLite `TEXT` 列,内容固定为 24 小时制、零填充的 `YYYY-MM-DD HH:MM:SS`(如 `2026-09-05 11:00:00`)。这个格式的字符串比较(`<`/`<=`/`>`/`>=`/`ORDER BY`)与时间先后顺序完全等价,现有 keyset 分页与范围过滤的 SQL 不用改比较逻辑,只改列类型。
- 备选 A——继续用 `INTEGER`,但换一种"本地纪元"编码(如把 `YYYYMMDDHHMMSS` 拼成整数):被否。字符串已经能达到同样的可比较性,还更直接可读,没必要为了省几个字节引入自定义编码。
- 备选 B——加一列存时区偏移,`time` 仍是 UTC 瞬时点:被否。这正是 proposal 里要否定的模型——用户的记忆是墙上钟时刻,不是"瞬时点 + 偏移量",加时区列只是把同一个错误模型做得更精致,没有解决根本问题,且时区偏移这个值本身也没人能可靠提供。
- 秒数精度足够(不引入毫秒/纳秒):记账场景不需要更细粒度,现状本就是秒。

### D2: 列名保持 `time` 不变

- 保持 `transactions.time` 这个列名,只改类型注释(从 `-- unix seconds` 改为说明新格式)。
- 备选——改名为 `occurred_at`/`local_time` 等更精确的名字:被否。列名 `time` 本身不隐含任何时区语义,改名除了让这次 diff 更大之外没有额外收益;真正需要澄清语义的地方是类型注释和各处 jsonschema 描述,不是列名。

### D3: Go 侧一律用 `string`,不引入 `time.Time`

- `store` 层、`tools` 层的 `Time`/`StartTime`/`EndTime` 字段类型统一为 Go `string`,格式校验用一个共享的正则或 `time.Parse("2006-01-02 15:04:05", s)`(仅用于格式校验,校验后仍以原字符串存储/传递,不转换成 `time.Time` 参与任何计算)。
- 备选——用 Go `time.Time`(配合 `time.Local` 或某个占位 `Location`)在内存里表示:被否。`time.Time` 内部始终携带一个 `Location`,任何格式化/比较操作都可能意外引入时区语义,与"完全无时区"的目标相悖;用纯字符串反而能保证任何时候都不会有代码不小心做了时区转换。

### D4: 分页游标的时间部分从 `int64` 改为 `string`,结构不变

- `mcp/internal/tools/transactions_cursor.go` 的 `searchTransactionsFilterFields.StartTime`/`EndTime`(`sql.NullInt64` → `sql.NullString`)与 `searchTransactionsCursor.LastTime`(`int64` → `string`)、`encodeSearchTransactionsCursor`/`decodeSearchTransactionsCursor` 的参数类型同步改为 `string`。游标仍是 base64url(JSON) 编码的不透明字符串,未签名(沿用原 design 的"游标是正确性 token,不是授权 token"这条决策),对外行为不变。

### D5: `timeline.go` 退化摘要删除时区换算,widget 删除 `Date` 时区转换

- `formatUnixDate(ts int64) string`(及其 `summaryZone = time.FixedZone("UTC+8", ...)`)整个删除,替换为直接对已存储的 `YYYY-MM-DD HH:MM:SS` 字符串取日期部分(前 10 个字符或按空格切分)。
- `timeline.html` 里 `dayKeyOf`/时间展示相关的 `new Date(unixSeconds * 1000)` + `Intl.DateTimeFormat(undefined, ...)` 全部删除,改为对 `t.time` 字符串做纯文本切分/格式化(取 `YYYY-MM-DD` 部分做分组键,取 `HH:MM` 部分展示),不经过任何 `Date` 对象,从根本上排除"按查看设备时区重新解释"的可能。

### D6: skill 侧的落地方式不体现为一个"转换步骤",而是直接产出目标格式

- 三份 `SKILL.md`(record/query/analysis)里原先"解析 → 心算/`date` 换算成 Unix 秒 → 写入"和"读出秒数 → `date -r`/`date -d @<ts>` 反查 → 展示"这两段各自独立的换算纪律,合并为一段:"解析 → 用 `date` 命令直接格式化输出 `YYYY-MM-DD HH:MM:SS` → 写入/展示"。相对日期锚点的来源见 D7。

### D7: 删除 `plugin-time-context-hook`,当前日期锚点改用宿主系统提示词自带的 `currentDate`

- **触发**:cowork / claude.ai 网页版等托管执行环境里,`time-context.sh` 实际运行在 Anthropic 的服务器上,系统时区是 UTC,不是用户实际所在地的 CST+0800——`date`/`date +%z` 在这些环境里给出的"本地时区"是错的,会让"今天"在 UTC 16:00–24:00(对应用户本地 0:00–8:00)这段时间算错一天。这是 `harden-record-skill-add-time-hook` 那次 change 里"时区取宿主系统本地时区即可"这条决策在托管执行场景下暴露出的缺陷。
- **选定**:不修 hook,直接删除整个 `plugin-time-context-hook` 能力(`plugin/hooks/time-context.sh`、`plugin/hooks/hooks.json` 里的 `SessionStart`/`UserPromptSubmit` 注册、对应的 spec)。原因:Claude Code / claude.ai 的系统提示词本身已经会注入当前日期(`currentDate`,见本会话上下文里的 `# currentDate` 段),这是宿主平台级的能力,不依赖 hook 执行所在的主机系统时钟,天然不受"hook 跑在服务器上"这个问题影响。tally 自己再跑一次 `date` 猜时区,只是在重复宿主已经提供、而且提供得更可靠的信息。相对日期表达("今天/昨天/上周五/3 天前")的解析,改为以宿主注入的 `currentDate` 为锚点用 `date` 命令计算,不再依赖 tally 自己的 hook。
- **备选 A(此前采纳、现推翻)——保留 hook,把判定时区的方式从"执行主机系统时钟"改成脚本内硬编码的 IANA 时区常量(如 `TZ='Asia/Shanghai' date ...`)**:能修好 cowork 场景下的 bug,但保留了一个本可以整体去掉的组件——多一个 hook 脚本、多一份需要随用户搬迁手动维护的时区常量,而宿主已经提供了同样(甚至更可靠)的信息。删除比"修好但保留"更符合项目一贯的极简立场。
- **备选 B——探测执行环境(本地 vs 云端)并分支处理**:被否,原因同上一版 D7——与项目"不做时区探测/地理定位"的立场冲突,且已经不需要,因为宿主本身的锚点就是对的。
- **代价**:依赖宿主 `currentDate` 注入的可靠性——如果某个执行表面(未来的新宿主/新接入方式)不提供这份注入,relative 日期解析会失去锚点。风险与兜底见下方 Risks。

## Migration Plan

1. 修改 `mcp/internal/store/schema.sql` 的 `time` 列类型与注释;`queries.sql` 里 `start_time`/`end_time`/`after_time` 相关参数注释同步更新。重新跑 sqlc 生成 `queries.sql.go`(类型从 `int64`/`interface{}` 变为 `string`/`sql.NullString`)。
2. 修改 `mcp/internal/tools/{transactions,transactions_cursor,analytics,timeline}.go` 里对应字段类型、jsonschema 描述、格式校验逻辑(替换 `txTime <= 0` 一类的数值校验)。
3. 修改 `mcp/internal/widgets/timeline.html`:删除所有 `Date`/`Intl.DateTimeFormat` 时区转换,改为字符串切分。
4. 修改三份 `plugin/skills/{record,query,analysis}/SKILL.md` 的时间纪律段落,把对 tally 自己 hook 的引用改为引用宿主自带的 `currentDate`;删除 `plugin/hooks/time-context.sh` 与 `plugin/hooks/hooks.json` 里的对应注册(D7)。
5. 更新 `openspec/config.yaml` 项目 context 里"不做任何时区处理——时间字段一直是纯 Unix 秒数"这句过时表述。
6. 手动清空/重建本地与生产的 SQLite 数据库文件(用户已确认不需要保留存量数据);`CREATE TABLE IF NOT EXISTS` 会在下次启动时按新 schema 建表。
7. 回滚策略:改动集中在类型与格式,没有中间状态需要兼容;如需回滚,恢复本 change 之前的 commit 并重建数据库文件即可,不涉及数据迁移脚本。

## Risks / Trade-offs

- **[跨时区绝对排序能力丧失]** 存 naive 本地时间后,两笔分别在不同时区记录的交易,`time` 字符串的先后顺序不再对应世界统一时间轴上的真实先后顺序(如"韩国 23:00" 和"中国 22:00" 实际上后者更早发生,但字符串比较会认为前者更早)→ 接受此权衡:proposal 里已论证个人记账场景几乎不需要跨时区的绝对瞬时排序,需要的是"用户记忆中的时刻"和"按自然日分组/汇总的正确性",这正是 naive 本地时间要优化的目标。
- **[agent 仍可能弄错锚点日期]** 即使锚点本身是对的,"今天是几号"之后的相对日期推算仍依赖 agent 正确使用 `date` 命令,不能 100% 消除解析错误 → 沿用现有的 record skill"预览确认"纪律作为兜底,与本 change 之前一致,不是本 change 引入的新风险。
- **[依赖宿主的 `currentDate` 注入]** D7 删除了 tally 自己的 hook,当前日期锚点完全依赖宿主平台注入 `currentDate` → 这是本 change 新引入的依赖;如果某个执行表面不提供这份注入,agent 会退回"没有锚点时先跑一次 `date` 取当前时刻"这条既有兜底规则(三份 SKILL.md 均保留了这条),不会完全没有锚点可用,但那种情况下会重新暴露"hook 跑在哪台机器上决定时区"的老问题——不属于本 change 能解决的范围,发生时再单独处理。
- **[存量数据全部丢弃]** 现有生产库中已记录的交易会在重建 schema 时清空 → 已与用户确认可接受;若日后需要保留,需另外手工导出旧数据再按新格式重新导入,不在本 change 范围内。
