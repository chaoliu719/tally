## Context

见 proposal.md - Why。补充当前实现事实:

- widget(`mcp/internal/widgets/timeline.html`)与服务端之间的每一次 `callServerTool` 都要穿过 `iframe → 宿主 → model-context 传输通道 → 服务端` 桥接,单次固定 2-5s,与 payload 大小无关;SQLite 查询本身亚毫秒。
- 现状:`ontoolresult` 拿到首屏 50 条(`timelineFirstPageSize`);随后 `loadLookups()` 并发两次 `callServerTool` 补分类/来源名并填筛选下拉;翻页越界触发 `search_transactions`;`filterApply` / `filterReset` 走 `reloadWithFilters()` 清空本地数据、带 `start_time/end_time/category_id/source_id` 参数重新联网分页。游标分页要求每页用完全一致的过滤参数重放,所以任何筛选变更都被迫从头联网。
- 已有 `search_transactions` 支持 `include_descendants`(选父类连带子孙),`list_categories` 返回 `parent_id`。
- 已有 supersession 机制:`BroadcastChannel("tally-transaction-timeline")`,更新的实例让旧实例 `superseded = true` 从而停止取数。
- 单用户、个人账本,规模数千条交易。

## Goals / Non-Goals

**Goals:**
- 打开面板后,分类/来源名字与筛选下拉零额外请求即可用。
- 筛选(时间/分类/来源)、翻页在数据取全后为纯本地操作,无网络往返。
- 后台把整份账本交易拉进 iframe,一次拉完,期间不阻塞首屏与翻页。
- 对接旧版服务端(结果无查找表)时仍可用,自动降级。

**Non-Goals:**
- 不改服务端分页/游标协议本身;`search_transactions` 的过滤参数保留给其它调用方,只是 widget 不再用它做筛选。
- 不解决"用户在别处改了交易、widget 不自动感知"——widget 刷新仍依赖对话里再产生一次工具调用(既有宿主机制,不在本 change 范围)。
- 不做增量/虚拟滚动等大数据量优化;若账本量级显著变大再单独提。

## Decisions

### D1: 分类/来源查找表随 `open_transaction_timeline` 结果返回

`OpenTransactionTimelineOutput` 新增:

```go
Categories []CategoryInfo `json:"categories"` // {id, name, parent_id}
Sources    []SourceInfo   `json:"sources"`    // {id, name}
```

复用 `list_categories` / `list_sources` 已有的行类型与查询(`deps.Q.ListCategories` / `ListSources`,按 `ledger_id`)。文本降级摘要不变——查找表只进结构化结果,不进 text block(避免污染不渲染 widget 的宿主的回答)。

**备选**:让 widget 继续自己拉 `list_categories`/`list_sources` 但并行化 + 提前触发。否决:仍是 2 次桥接往返,省不掉那 3-5s;首屏结果里带上是零成本(同一次 DB 事务多两条 SELECT)。

### D2: 首屏后台一次性拉全 + 本地全量过滤

`timelineFirstPageSize` 50 → 200。首屏渲染后立即 `void drainAll()`:循环 `fetchNextChunk()`(不带任何过滤参数)直到 `doneFetching || superseded`。`allTxns` 即全量。

筛选视图:引入 `visibleTxns()` = `allTxns.filter(matchesFilters)`。所有渲染/分页函数(`renderPage` / `computeDayTotals` / `pageCount` / `hasNextPage` / `lastPageIdx` / 翻页按钮 disable 逻辑)改为基于 `visibleTxns()` 的结果(计算一次缓存到局部变量,避免每次重算)。

过滤条无"应用"按钮:`filterStart` / `filterEnd` / `filterCat` / `filterSrc` 各自的 `change` 事件都绑到同一个 `commitFilters()` —— 读四个输入的当前值 → 更新 `filters` / `filtersActive` / `catFilterIds` → `pageIdx = 0` → `refreshView()` → `updateMeta()` → `renderPage()`,纯本地、瞬时。`filterReset` 清空四个输入再调 `commitFilters()`。删除 `reloadWithFilters()` 的清空与联网。

**备选**:保留"应用"按钮做一次性提交。否决:过滤全本地后单次过滤成本可忽略,即时反馈体验更好,少一个必须点的控件(用户明确要求)。日期 input 的 `change` 只在失焦/选定后触发,不会边打字边抖动。

### D4: 两个 widget bug(测试中发现)

- **深色→全屏变浅色**:`onhostcontextchanged` 不只在主题切换时触发,展示模式切换(进/出全屏)也会触发,且该上下文可能不带 `theme`。旧代码 `ctx && applyTheme(ctx.theme)` → `applyTheme(undefined)` → `classList.toggle("dark", false)` 把深色清掉,且退出全屏不会再补发主题,所以一直是浅色。修复:`applyTheme` 只认 `"light"` / `"dark"`,其它一律忽略(不动当前配色)。
- **「筛选」点击无反应**:过滤条靠元素的 `[hidden]` 属性收起,但 `.filterbar { display: flex }` 是作者样式,在 UA 的 `[hidden] { display: none }` 不带 `!important` 的浏览器里会盖过它 —— `hidden` 形同虚设,`toggle` 也就"没反应"。修复:`.filterbar` 默认 `display: none`,`.filterbar.open` 才 `display: flex`,`filterToggle` 切 `.open` class 并同步 `aria-expanded`。顺带给按钮加 caret 与活跃筛选圆点(用户要求"改样子")。
- **进全屏后「全屏」按钮还在**:按钮可见性此前只在 init 时按 `availableDisplayModes` 判一次。改为 `syncDisplayMode(ctx)` 挂在 `onhostcontextchanged` 上,按 `ctx.displayMode`/`ctx.mode` 是否为 `fullscreen` 实时隐藏/恢复;进全屏即隐藏,退出全屏由宿主自己负责,回到内联时再显示。

### D5: widget 资源加载慢(~10s)

`ui://` 资源(内联 ext-apps runtime 后约 380KB)每次打开面板 / 刷新页面都要经 `iframe → 宿主 → model-context → 服务端` 重新 `resources/read` 一次;go-sdk 默认 `TTLMs=0` = 立即过期,宿主每次都全量重取。

- **选定:给 `resources/read` 结果设 `TTLMs`(5min)+ `CacheScope="public"`**。宿主在 TTL 内复用缓存副本,一个工作会话里的反复开关/刷新不再重拉。5min 是折中:够短,新部署几分钟内自然生效(用户仍可硬刷新)。TTL 值 `widgetResourceTTL` 常量,可调。
- **否决:Caddy 层 gzip/zstd**。MCP streamable-HTTP 的 POST /mcp 响应 Content-Type 是 `text/event-stream`(即便 `resources/read` 这种一问一答),Caddy `encode` 对 SSE 主动跳过压缩,`match` 也盖不过;强上 SSE 压缩有破坏流式的风险。
- **否决(本轮):缩小 bundle**。337KB 的 vendored `ext-apps-app-with-deps.js` 是大头,tree-shake/换精简构建有回归风险,留作后续。
- **否决:把 bundle 拆成独立可缓存子资源**。iframe CSP 禁止 module import / 外部脚本,必须内联。

`nextBtn` / `lastBtn`:取数只为"后台还没拉完时补齐",逻辑简化为"若 `!doneFetching` 先 `await drainAll()`,再纯本地跳页"。

**子孙分类展开**:在 `catById`(已有 `parentId`)基础上构建 `descendantsOf(catId)` —— 一次性 BFS 出 `parentId → children[]` 邻接表,选中分类时收集其子树 id 集合,`matchesFilters` 里按 `idSet.has(t.category_id)` 判断。等价于服务端 `include_descendants=true`。

**日期比较**:`filters.start`/`filters.end` 存 `YYYY-MM-DD`(去掉现有代码给 date input 拼的 ` 00:00:00` / ` 23:59:59` 后缀,改为直接比较日期前缀):`dayKeyOf(t.time) >= start && dayKeyOf(t.time) <= end`。字典序 == 时间序,与仓库既有的"在定宽字符串字段上直接切片"纪律一致(design 里 D1/D5)。

**备选 A**:保留增量分页,只在"用户首次应用筛选"时 `ensureAllFetched()`,之后本地。否决:实现上要维护"已全量 / 未全量"两套翻页路径,复杂度高于直接开场就拉全;且开场拉全同时把"页尾"和翻页也变快,正好命中用户主诉的"卡"。

**备选 B**:不拉全,筛选也走增量——"下一页"时对已取回部分做本地过滤,不够一页再取。否决:页数、"共 N 笔"在数据取全前都是估计值,交互抖动;与"纯本地、瞬间"的目标背离。

### D3: 降级路径

`ontoolresult` 解析 payload 时,若 `categories`/`sources` 缺失或非数组 → 保留现有 `loadLookups()` 调用作为回退;若存在 → 直接填 `catById`/`srcById` 并 `populateFilterOptions()`,不调 `loadLookups()`。`drainAll()` 与降级无关,始终执行。

## Risks / Trade-offs

- **[整份账本进 iframe 内存]** → 单用户数千条、每条是小 JSON 对象,量级 ~百 KB 到个位 MB,浏览器无压力。文档里明确记为"账本量级显著变大时重新评估"的触发点。
- **[后台 drain 期间用户就改了筛选]** → `view` 基于当时已取回的部分,`drainAll` 完成后再 `renderPage()` 一次刷新(drain 循环结束时无条件重绘当前页)。页码/条数在 drain 完成前对"很久以前"的筛选结果可能偏小,可接受(与旧版"还没滚到"行为一致),drain 通常几秒内结束。
- **[drain 把整个账本一次性拉取,对话里多次打开会重复全量拉取]** → supersession 让旧实例停;每次打开是一轮全量桥接调用(200 条/页 → 数千条约 10~20 次 `search_transactions`)。比旧版(2 次 lookup + 按需翻页)总请求数多,但都在后台、不阻塞交互,且用户实际感知的每一次操作都不再等待。若成为问题,可把首屏页 size 再调大以减少轮数。
- **[旧版服务端 / 结果被 text-only 投递]** → `extractFirstPagePayload` 已能从 text block 解析 JSON;新字段走同一路径。缺字段则降级,不报错。
- **[`search_transactions` 无 limit 上限风险]** → 保持每页 `limit: 100`(与现状一致),仅首屏由服务端返回 200;不改。

## Migration Plan

1. 服务端加字段 → 构建镜像 → ssh aliyun `docker compose pull && up -d` → `curl /healthz`(见 memory: tally-mcp deploy flow)。
2. widget HTML 内嵌在二进制里,随同一镜像发布;宿主下次打开面板即取到新版资源。
3. 回滚:字段是纯新增,旧 widget 忽略未知字段;新 widget 对无字段结果自动降级。回退镜像即可,无数据迁移。
