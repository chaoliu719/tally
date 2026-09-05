## 1. 数据库与生成代码

- [x] 1.1 修改 `mcp/internal/store/schema.sql`:`transactions.time` 列类型从 `INTEGER` 改为 `TEXT`,注释更新为说明 `YYYY-MM-DD HH:MM:SS` 本地日期时间格式;验证:`grep` 确认列定义与注释已更新
- [x] 1.2 修改 `mcp/internal/store/queries.sql` 中 `start_time`/`end_time`/`after_time` 相关的注释,说明其现为同一格式的字符串;验证:`grep -n "unix" mcp/internal/store/queries.sql` 无匹配
- [x] 1.3 重新生成 `mcp/internal/store/queries.sql.go`(sqlc),确认 `Time`/`StartTime`/`EndTime`/`AfterTime`/`EarliestTime`/`LatestTime` 等字段类型变为 `string`/`sql.NullString`/`interface{}`;验证:`go build ./mcp/...` 通过
- [x] 1.4 更新 `mcp/internal/store/models.go` 里 `Transaction.Time` 字段类型为 `string`(如未随 sqlc 重新生成自动更新);验证:`go build ./mcp/...` 通过

## 2. Go 工具层

- [x] 2.1 修改 `mcp/internal/tools/transactions.go`:`CreateTransactionInput.Time`/`UpdateTransactionInput.Time`/输出结构体的 `Time` 字段类型改为 `string`,jsonschema 描述改为说明 `YYYY-MM-DD HH:MM:SS` 本地日期时间格式;验证:`go build ./mcp/...` 通过
- [x] 2.2 在 `validateTransactionInput`(或新增校验函数)中,把 `txTime <= 0` 的数值校验替换为对 `YYYY-MM-DD HH:MM:SS` 格式的字符串校验(建议用 `time.Parse("2006-01-02 15:04:05", s)` 仅做格式校验,不转换参与计算,校验失败返回明确错误);验证:单元测试覆盖合法格式、缺时间部分、非法月日、带时区后缀等非法输入
- [x] 2.3 修改 `SearchTransactionsInput.StartTime`/`EndTime` 及 `analytics.go` 里 `GetFinancialSummaryInput.StartTime`/`EndTime` 类型改为 `string`,复用 2.2 的格式校验;验证:单元测试覆盖非法格式被拒绝的场景
- [x] 2.4 修改 `mcp/internal/tools/transactions_cursor.go`:`searchTransactionsFilterFields.StartTime`/`EndTime` 改为 `sql.NullString`,`searchTransactionsCursor.LastTime` 改为 `string`,`encodeSearchTransactionsCursor`/`decodeSearchTransactionsCursor` 签名同步调整;验证:现有游标编解码单元测试更新后通过
- [x] 2.5 修改 `mcp/internal/tools/timeline.go`:删除 `summaryZone`/`formatUnixDate`,改为直接从 `YYYY-MM-DD HH:MM:SS` 字符串截取日期部分用于退化摘要;验证:`open_transaction_timeline` 相关单元测试通过,摘要文本不再出现 `(UTC+8)`
- [x] 2.6 跑通 `mcp` 模块全部现有单元测试,按需更新测试夹具里的时间值为新字符串格式;验证:`go test ./mcp/...` 全绿

## 3. Widget

- [x] 3.1 修改 `mcp/internal/widgets/timeline.html`:删除 `dayKeyOf`/时间展示逻辑中的 `new Date(unixSeconds * 1000)` 与 `Intl.DateTimeFormat`,改为对 `t.time` 字符串做纯文本切分(日期部分做分组键、`HH:MM` 部分展示);验证:`go test ./mcp/internal/widgets/...`(含快照/字符串测试)通过
- [ ] 3.2 手动在支持 Apps 表面的宿主(或本地起一份测试数据)中打开时间线面板,确认分组与展示的时间与写入值一致,且不随浏览器时区设置变化;验证:切换浏览器/系统时区后面板显示不变

## 4. Skill 与 Hook 文档

- [x] 4.1 修改 `plugin/skills/record/SKILL.md`"时间字段没有时区语义"一节,改为"解析为本地日期时间字符串,不做 Unix 秒换算",并把"tally plugin 的 time-context hook 会…注入"的措辞改为引用宿主(Claude Code / claude.ai)系统提示词自带的当前日期上下文;验证:与 `specs/plugin-record-skill/spec.md` 的 MODIFIED 内容一致,`grep -n "time-context hook" plugin/skills/record/SKILL.md` 无匹配
- [x] 4.2 修改 `plugin/skills/query/SKILL.md` 对应的两条时间换算纪律,合并为"解析/展示均使用本地日期时间字符串,不做秒数换算",并去掉对 `plugin-time-context-hook` 的引用;验证:与 `specs/plugin-query-skill/spec.md` 的 ADDED/REMOVED 内容一致,`grep -n "plugin-time-context-hook\|time-context hook"` 对该文件无匹配
- [x] 4.3 修改 `plugin/skills/analysis/SKILL.md` 里"自行计算月份边界的 Unix 秒数"相关表述,改为本地日期时间字符串;验证:与 `specs/plugin-analysis-skill/spec.md` 的 MODIFIED 内容一致
- [x] 4.4 删除 `plugin/hooks/time-context.sh` 与 `plugin/hooks/hooks.json`(若该目录下无其他 hook,整个 `plugin/hooks/` 目录一并删除);验证:`ls plugin/hooks` 报不存在(或目录为空/仅剩其他 hook),`grep -rn "time-context" plugin/` 无匹配
- [x] 4.5 确认删除 hook 后 `plugin/.claude-plugin`(或 plugin 清单)里没有遗留对 `hooks/hooks.json` 的显式引用;验证:插件在本地加载/校验(如有校验命令)不报缺文件错误

## 5. 项目级文档与收尾

- [x] 5.1 更新 `openspec/config.yaml` 项目 context 中"不做任何时区处理——时间字段一直是纯 Unix 秒数"一句,改为描述新的本地日期时间字符串模型;验证:`grep -n "Unix 秒" openspec/config.yaml` 无匹配
- [x] 5.2 清空/重建本地开发用 SQLite 数据库文件,确认新 schema 生效;验证:`create_transaction` 写入一笔交易后 `sqlite3` 直接查看 `time` 列为 `YYYY-MM-DD HH:MM:SS` 字符串(用一次性临时 DB 验证过:`typeof(time)=text`,值原样为 `2026-09-05 11:00:00`;未触碰生产库)
- [ ] 5.3 端到端手测:记一笔交易(如"今天 11:00 买咖啡")→ `search_transactions`/`get_transaction` 查询 → 打开时间线 widget,确认全链路时间显示一致且与写入值相符;验证:人工核对三处展示的时刻完全一致
- [ ] 5.4 在 cowork / claude.ai 网页版会话中验证:不依赖 tally 自己的 hook 时,agent 解析"今天"仍然正确(与用户实际所在地日期一致,不是服务器所在地的 UTC 日期);验证:在该会话里让 agent 报出它认为的"今天日期",与用户实际当地日期核对一致
