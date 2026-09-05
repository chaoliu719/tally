## 1. 过滤条 UI

- [x] 1.1 在 `mcp/internal/widgets/timeline.html` 的 header 中加入「筛选」开关按钮，点击切换一条过滤条(起始日期、结束日期、分类下拉、来源下拉、重置/应用按钮)的显示/隐藏；验证：本地打开该 HTML 文件，点击按钮能看到过滤条展开与收起
- [x] 1.2 过滤条与暗色主题的现有 CSS 变量(`--bg`/--fg`/`--line` 等)保持一致，浅色/深色下均可读；验证：切换 `document.documentElement.classList` 的 `dark` 类，肉眼检查过滤条配色

## 2. 分类/来源下拉数据

- [x] 2.1 在现有 `loadLookups()` 完成后，用已缓存的 `catById`/`srcById` 填充分类、来源下拉选项：分类下拉中顶层分类平铺、子分类缩进显示在其父分类之后；验证：对一个有多级分类的账本打开面板，下拉选项顺序与缩进符合预期
- [x] 2.2 下拉默认选项为「不限分类」「不限来源」，对应过滤参数留空；验证：不选择任何分类/来源时应用过滤，请求不带 `category_id`/`source_id`

## 3. 后端：分类过滤含子孙分类

- [x] 3.1 探路验证：写一个最小 Go 测试，确认 `modernc.org/sqlite` 是否支持 JSON1 的 `json_each`（例如对 `SELECT value FROM json_each('[1,2,3]')` 断言返回 1/2/3）；验证：测试通过并输出预期值。根据结果决定 3.2 走 `json_each` 动态 IN 列表方案，还是回退到手写（不经 sqlc）拼 `IN (?,?,?...)` 占位符的方案 —— **结果：支持，走 json_each 方案**
- [x] 3.2 在 `mcp/internal/store/queries.sql` 的 `SearchTransactions`/`SearchTransactionsDesc` 中，让 `category_id` 过滤支持匹配一组 id（该分类自身，加上当 `include_descendants` 为真时的全部子孙 id），按 3.1 的结论选择具体实现；跑 `sqlc generate` 更新生成代码；验证：`cd mcp && go build ./...` 通过
- [x] 3.3 `search_transactions` 工具的 input schema 新增可选字段 `include_descendants`（bool，默认 `false`）；handler 在其为 `true` 且 `category_id` 非空时，调用现有的 `ListCategoryDescendantIDs` 算出子孙 id 集合并入过滤范围；验证：新增单测覆盖"精确匹配（默认/false）"与"含子孙（true）"两种情况
- [x] 3.4 确认不传 `include_descendants`（或传 `false`）时的行为与改动前完全一致；验证：`search_transactions` 现有单测全部通过，且 `plugin/skills/optimize` 文档描述的分类合并流程（依赖 `search_transactions(category_id=A)` 精确匹配 A 自身）逻辑不受影响，无需修改该 skill 文件

## 4. 过滤生效与重新拉取

- [x] 4.1 点击「应用」时，把过滤条当前值(日期转换为 `YYYY-MM-DD HH:MM:SS`，起始补 `00:00:00`、结束补 `23:59:59`)提交为活跃过滤条件，清空已加载的交易数组与分页游标，回到第 1 页，收起过滤条；验证：设置一个已知会缩小结果集的过滤条件后应用，面板只展示匹配的交易且从第 1 页开始
- [x] 4.2 面板向 `search_transactions` 发起的请求(首次与翻页懒加载)统一带上当前活跃过滤条件中非空的 `source_id`/`category_id`/`start_time`/`end_time`；验证：翻到第 2 页及以后仍只返回符合过滤条件的交易
- [x] 4.3 点击「重置」清空过滤条中的所有输入与活跃过滤条件，重新拉取该账本全部交易；验证：设置过滤后再重置，面板恢复展示未过滤的完整交易列表
- [x] 4.4 选中分类下拉(无论顶层或子分类)时，面板向 `search_transactions` 固定附带 `include_descendants: true`，让该分类的全部子孙分类下的交易一并被包含(依赖第 3 组的后端支持)；验证：对一个有子分类的父分类应用过滤，归类在其子分类下的交易也会出现

## 5. 统计文案与空状态

- [x] 5.1 活跃过滤条件存在时，顶部统计文案从「共 N 笔」切换为提示"已筛选"的文案；清空过滤后恢复显示未过滤总数；验证：应用/重置过滤，观察顶部文案切换
- [x] 5.2 过滤后无匹配交易时，空状态提示文案与账本本身为空时的提示区分开(如"没有符合当前条件的交易" vs "这个账本还没有交易")；验证：设置一个必然无匹配结果的过滤条件，检查提示文案

## 6. 验证与收尾

- [x] 6.1 `cd mcp && go build ./...` 通过(确认 `go:embed` 的 HTML 改动不破坏构建)
- [x] 6.2 对 `timeline.html` 内联的 `<script type="module">` 做一次 `node --check` 语法校验
- [x] 6.3 `cd mcp && go test ./...` 通过，覆盖第 3 组新增的 `include_descendants` 单测
- [ ] 6.4 在 claude.ai cowork 环境中实际打开一次 widget，走一遍时间范围/分类(含选父类看子孙交易是否出现)/来源过滤、组合过滤、重置的交互，确认与 `specs/transaction-timeline-widget/spec.md` 中的 Scenario 一致
