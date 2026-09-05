## 1. 服务端:查找表进结果

- [x] 1.1 `OpenTransactionTimelineOutput` 新增 `Categories []CategoryInfo` 与 `Sources []SourceInfo` 字段(复用 `tools.CategoryInfo` / `tools.SourceInfo`),补 jsonschema 说明。验证:`go build ./...` 通过。
- [x] 1.2 `openTransactionTimeline` 中查询该账本的分类与来源(复用 `deps.Q.ListCategories` / `ListSources` + `toCategoryInfo` / `toSourceInfo`),填入结果的两个新字段;文本降级摘要保持不变。验证:新增 `internal/tools/timeline_test.go` 断言含父子分类与来源的账本调用 `open_transaction_timeline` 时两组查找表非空、`parent_id` 正确、空账本返回非 nil 空切片;`go test ./internal/tools/` 通过。
- [x] 1.3 上调 `timelineFirstPageSize`(50 → 200),`nextCursor` 计算不变(基于该常量)。验证:`e2e_transaction_timeline_paging_test.go` 不校验首屏条数,`e2e_transaction_timeline_widget_test.go` 与全套 e2e 通过。

## 2. widget:首屏用查找表 + 去掉开场取数

- [x] 2.1 `ontoolresult`:新增 `ingestInlineLookups(payload)` 解析 `categories` / `sources`,存在则同步填 `catById`(含 `parentId`)/ `srcById`、`rebuildCategoryTree()`、`populateFilterOptions()`,首屏即渲染名字与下拉,不调 `loadLookups()`。验证:`widgets_test.go` 断言 HTML 含 `ingestInlineLookups`;`go test ./internal/widgets/` 通过。
- [x] 2.2 保留 `loadLookups()` 仅在 `ingestInlineLookups` 返回 false(结果缺查找表)时作为降级回退;`loadLookups` 结束也 `rebuildCategoryTree()`。验证:代码注释说明降级条件;`list_categories` / `list_sources` 仍在 HTML 中(测试断言)。

## 3. widget:后台拉全 + 本地过滤视图

- [x] 3.1 首屏渲染后 `void drainAll()`:循环 `fetchNextChunk()`(不带任何过滤参数)直到 `doneFetching || superseded`,每块后 `renderControls()`、结束 `renderPage()`;`drainAll` 复用同一 in-flight promise 以便按需 `await`。验证:`hasNextPage()` 在 `!doneFetching` 时恒为真使「下一页/页尾」在 drain 期间保持可用,drain 完成后到底置灰。
- [x] 3.2 新增 `view` + `refreshView()`(`filtersActive ? allTxns.filter(matchesFilters) : allTxns`);`renderPage` / `renderControls` / `computeDayTotals` / `pageCount` / `hasNextPage` / `lastPageIdx` 全部改为基于 `view`。验证:`go test ./internal/widgets/` 通过 + JS 解析检查通过。
- [x] 3.3 `childrenByParent` 邻接表 + `descendantIdsOf(catId)`(DFS 子树 id 集合,含自身);`matchesFilters` 分类判断用提交筛选时算好的 `catFilterIds`;叶子分类退化为精确匹配。验证:`widgets_test.go` 断言 HTML 含 `descendantIdsOf` / `matchesFilters`。
- [x] 3.4 日期过滤:`commitFilters` 直接存 `filterStart.value` / `filterEnd.value` 的 `YYYY-MM-DD`(去掉 ` 00:00:00` / ` 23:59:59` 拼接);`matchesFilters` 按 `dayKeyOf(t.time)` 与边界做字典序比较(含边界)。验证:字典序等价时间序,边界日期被包含。

## 4. widget:筛选与翻页去联网

- [x] 4.1 过滤条去掉「应用」按钮:`filterStart` / `filterEnd` / `filterCat` / `filterSrc` 的 `change` 事件各自绑到 `commitFilters()`(读四个输入当前值 → 更新 `filters` / `filtersActive` / `catFilterIds` → `pageIdx = 0` → `refreshView()` → `updateMeta()` → `renderPage()`);`filterReset` 清空输入后调 `commitFilters()`;删除 `reloadWithFilters()` 及其 `search_transactions` 重拉,移除 `#filterApply` 元素与 `.filterbar button.primary` 样式。验证:HTML 中不含 `filterApply` / "应用";`widgets_test.go` 断言过滤条无 primary 按钮;`go test ./internal/widgets/` 通过。
- [x] 4.2 `nextBtn` / `lastBtn`:若 `!doneFetching` 先 `await drainAll()`,再基于 `view` 的 `lastPageIdx()` 纯本地跳页。验证:drain 未完成时点「页尾」经 `drainAll` 完成后跳到最后一页。
- [x] 4.3 `updateMeta()`:`filtersActive` 时显示 `已筛选 · ${view.length} 笔`;否则 `共 ${total} 笔`;空结果区分 `没有符合当前条件的交易` vs `这个账本还没有交易`。验证:`widgets_test.go` 断言两种空态文案都在 HTML 中。

## 4b. widget bug 修复(测试中发现)

- [x] 4b.1 深色主题下进入/退出全屏变浅色:`applyTheme` 只接受 `"light"` / `"dark"`,`onhostcontextchanged` 里不含主题字段的上下文直接忽略,不再 `toggle("dark", false)`。验证:深色下 `app.requestDisplayMode({mode:"fullscreen"})` 往返后仍为深色。
- [x] 4b.2 「筛选」控件点击无反应:过滤条改用显式 `.filterbar.open` class 控制展开/收起(默认 `display:none`),不再依赖会被 `.filterbar{display:flex}` 覆盖的 `[hidden]` 属性;`filterToggle` 切 `.open` 并同步 `aria-expanded`;按钮加 caret(▾/▴)与活跃筛选圆点。验证:`widgets_test.go` 断言 HTML 含 `.filterbar.open` 与 `aria-expanded`;JS 解析通过。

## 5. 校验与收尾

- [x] 5.1 更新 `e2e_transaction_timeline_widget_test.go`(断言结果含 inline categories/sources)、`widgets_test.go`(新增 `ingestInlineLookups` / `drainAll` / `descendantIdsOf` / `matchesFilters` / 过滤空态文案断言);`internal/tools/timeline_test.go` 覆盖结果查找表与空账本。验证:`go test ./...` 全绿。
- [x] 5.2 `openspec validate timeline-widget-local-filtering --strict` 通过。
- [ ] 5.3 归档时用 openspec-archive 把 delta 同步进 `openspec/specs/transaction-timeline-widget/spec.md`。验证:归档后主 spec 三个 Requirement 反映新行为,`openspec list --specs` 无警告。
