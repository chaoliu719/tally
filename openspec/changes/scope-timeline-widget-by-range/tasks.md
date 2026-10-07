## 1. 验证门槛:面板是否依赖文本块里的 JSON

- [x] 1.1 阅读 `mcp/internal/widgets` 中 `extractFirstPagePayload` 的取数顺序,并在 e2e 里构造"只有结构化结果、文本不含 JSON"的工具结果喂给面板,确认首屏能正常渲染。验证:新增/调整的测试通过;结论(依赖 / 不依赖文本 JSON)写进 design.md D4 的验证门槛下;若依赖,按 D4 降级方案调整本变更后续第 3 组任务再继续。

## 2. 服务端:范围参数、校验与统计

- [x] 2.1 `queries.sql` 新增 `SummarizeTimelineScope`(`ledger_id` / `source_id` / `category_ids_json` / `start_time` / `end_time`,按 `currency` 分组返回 `COUNT(*)`、收入合计、支出合计),重新生成 sqlc 代码。验证:`go build ./...` 通过;`internal/store` 层测试覆盖按时间、按来源、按分类集合、多币种分组、空范围。
- [x] 2.2 `OpenTransactionTimelineInput` 新增 `start_date` / `end_date` / `category_id` / `source_id`;实现严格日期解析(`2006-01-02`)、`start_date > end_date` 报错、`GetCategory` / `GetSource` 校验属于该账本;日期换算成 `00:00:00` / `23:59:59` 的 `start_time` / `end_time`;`category_id` 用 `ListCategoryDescendantIDs` 展开。验证:`timeline_test.go` 覆盖非法日期格式、开始晚于结束、分类/来源不属于该账本、只传一端日期、不传任何范围参数(行为与此前一致)。
- [x] 2.3 首屏查询 `SearchTransactionsDesc` 与游标指纹(`searchTransactionsFilterFields`)带上范围条件,与 `searchTransactions` 组装指纹的方式一致。验证:`timeline_test.go` 断言带范围时首屏只含范围内交易(含 9 月 30 日 23:59:59、不含 10 月 1 日 00:00:00),且用返回的 `next_cursor` 加同样参数调用 `search_transactions` 能续页、改动任一范围参数则被拒。
- [x] 2.4 结果新增 `scope`(见 design D2),用 `SummarizeTimelineScope` 填 `Count` 并得到按币种收支合计;不再调用 `TransactionStats` 作为展示用总数。验证:`timeline_test.go` 断言 `scope` 回显各字段、`Count` 等于范围内实际条数、范围外交易不计入。
- [x] 2.5 文本摘要重写:生效范围描述、范围内条数、按币种(`currency.FormatMajor`)的收入与支出合计、"需要明细可用带同样范围的 `search_transactions`"提示;不再拼接交易 JSON(按第 1 组结论);范围内无交易时说明"该范围内没有交易"并带上范围。验证:`timeline_test.go` 断言文本不含任何单笔明细、多币种分列不相加、空范围文案正确。
- [x] 2.6 更新工具 `Description` 与各输入字段的 `jsonschema` 说明,写成给 Agent 看的规则(只加载指定范围、范围外不加载、不传即不限制、换范围需重新打开、日期含当天)。验证:`go test ./internal/tools/` 通过;`mcp/README.md` 中该工具的说明同步更新,文中示例参数可对照代码核对。

## 3. widget:范围展示、取数与筛选约束

- [ ] 3.1 面板读取 `scope`:后台 `drainAll` 每一页 `search_transactions` 的参数用 `scope` 回传的 `start_time` / `end_time` / `category_id` / `source_id` / `include_descendants`(不自行拼日期);缺 `scope` 时按"全部"处理。验证:`widgets_test.go` 断言 HTML 含对 `scope` 的读取;e2e 构造 9 月范围且账本含范围外交易,断言面板取回的交易恰为范围内交易(`e2e_transaction_timeline_paging_test.go`)。
- [ ] 3.2 顶部改为"范围 + 条数"(见 design D5):无筛选时显示范围内条数,有活跃筛选时条数改为本地匹配数、范围描述不变;不再使用账本总数 `total`。验证:`widgets_test.go` 断言新文案与对 `total` 的移除;手动在预览页(`preview.js`)检查两种状态。
- [ ] 3.3 预置的分类/来源显示为只读标签(分类附"含子分类"),隐藏对应下拉;未预置的保持下拉;`filterReset` 只清用户设置的输入、不动预置范围。验证:`widgets_test.go` 断言存在只读标签逻辑与重置不清范围;预览页分别以"预置分类""预置来源""都不预置"三种 `scope` 手动核对。
- [ ] 3.4 日期输入的 `min` / `max` 取自 `scope.start_date` / `end_date`(某端未设则不加)。验证:`widgets_test.go` 断言该约束逻辑;预览页手动核对范围外日期不可选。
- [ ] 3.5 空状态三分:账本为空、范围内为空(`该范围内没有交易` + 范围)、筛选后为空。验证:`widgets_test.go` 断言三种文案都在 HTML 中;`go test ./internal/widgets/` 通过。
- [ ] 3.6 同步 e2e:`e2e_transaction_timeline_widget_test.go` 覆盖带范围打开、结果含 `scope`、旧式无 `scope` 结果的降级。验证:`go test ./cmd/tally-mcp/` 全绿。

## 4. skill:使用说明与默认行为

- [ ] 4.1 `plugin/skills/query/SKILL.md` 新增"用时间线浏览"一节(见 design D6 与 `plugin-query-skill` delta),并在 `description` 里补上"打开时间线 / 翻翻账本"一类触发说法。验证:通读该节,对照 delta 的每个 Scenario(本月、无范围默认 30 天、明确要全部、分类/来源先查 id、换范围重新打开、`date` 换算)逐条能在文字里找到对应规则;文中 `date` 示例命令在本机能跑出 `YYYY-MM-DD`。
- [ ] 4.2 `plugin/README.md` 如有 skill 触发说明或工具清单提及时间线,同步更新。验证:`Grep` 该 README 中相关处与新行为一致。

## 5. 校验与收尾

- [ ] 5.1 `go test ./...` 全绿,`openspec validate scope-timeline-widget-by-range --strict` 通过。验证:两条命令均退出码 0。
- [ ] 5.2 用本机 stdio/HTTP 方式起服务,依次以(a)只传 `ledger_id`、(b)9 月范围、(c)9 月 + 分类、(d)非法范围 调用 `open_transaction_timeline`,检查文本摘要与结构化结果。验证:四种结果与 spec 的对应 Scenario 一致,(d)返回明确错误。

## Workflow follow-up

- 归档本变更时,用 openspec-archive 把两份 delta 同步进 `openspec/specs/transaction-timeline-widget/spec.md` 与 `openspec/specs/plugin-query-skill/spec.md`。
- 归档后手工修改 `openspec/specs/transaction-timeline-widget/spec.md` 的 Purpose(delta 不能改 Purpose):把"浏览某个账本的全部交易——直到最早一条"改为"浏览某个账本在所选范围内的交易",其余句子保持不变。
- 按既有流程构建镜像、部署并 `curl /healthz`。
