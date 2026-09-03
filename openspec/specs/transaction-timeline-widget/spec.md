# transaction-timeline-widget Specification

## Purpose
让唯一用户在支持 MCP Apps 表面的宿主里,以一个内联的交互式面板从新到旧、按天分组、滚动分页地浏览某个账本的全部交易——直到最早一条,且浏览过程中的交易数据不进入 Agent 的对话上下文。同时约束 tally-mcp 托管此类 `ui://` widget 资源时的通用规则(自包含、无外部请求、宿主无此能力时降级为工具文本)。

## Requirements

### Requirement: 打开交易时间线 widget
用户 SHALL 能够通过 `open_transaction_timeline` 工具,指定 `ledger_id`,让宿主渲染一个交互式交易时间线面板。该工具的结果 `_meta` SHALL 声明时间线 widget 的 `ui://` 资源 URI;工具本身的文本内容 SHALL 是一段可独立阅读的摘要(该账本交易总条数、最早与最新交易的日期、以及最新一页交易的简要列表),使不支持 Apps 表面的宿主忽略 `_meta.ui` 后仍能得到有意义的回答。该工具 SHALL 是只读的,不创建、修改或删除任何数据。

#### Scenario: 在支持 Apps 表面的宿主打开
- **WHEN** 用户在支持 MCP Apps 表面的宿主中触发 `open_transaction_timeline` 并指定一个已存在的账本
- **THEN** 宿主拉取声明的 `ui://` 资源并在对话内渲染为 iframe 面板,面板首屏展示该账本最新的一页交易

#### Scenario: 在不支持 Apps 表面的宿主打开
- **WHEN** 同一工具在不支持 Apps 表面的宿主(或 MCP Inspector)中被调用
- **THEN** 用户看到工具返回的文本摘要(总条数、日期范围、最新一页列表),不报错

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `open_transaction_timeline` 指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误,不渲染面板

### Requirement: 时间线面板按天分组、倒序、滚动分页
时间线面板 SHALL 将交易按其发生日期(以用户本地时区解释 `time` 的 unix 秒)分组,日期段从最新到最早排列,每段内交易也按时间倒序。面板 SHALL 在用户向下滚动接近已加载内容底部时,自动通过 `callServerTool` 调用 `search_transactions`(`newest_first=true`,带上一页返回的 `cursor`)拉取并追加下一页,直到 `search_transactions` 不再返回 `next_cursor`,此时面板 SHALL 显示已到最早一条的明确提示。翻页请求 SHALL 不经过 Agent、不把交易数据写入对话上下文。

#### Scenario: 滚动加载更多
- **WHEN** 用户在面板内向下滚动到接近当前已加载交易的底部,且该账本还有更早的交易未加载
- **THEN** 面板自动拉取下一页并追加到列表末尾,已显示的交易与滚动位置不跳动

#### Scenario: 滚动到最早一条
- **WHEN** 用户持续向下滚动,面板已加载到该账本最早的一条交易
- **THEN** 面板停止发起翻页请求,并显示"已到最早一条"之类的明确结束提示

#### Scenario: 账本为空
- **WHEN** 对一个没有任何交易的账本打开时间线面板
- **THEN** 面板显示空状态提示,不发起翻页请求,不报错

#### Scenario: 跨天边界分组
- **WHEN** 加载进来的交易中存在同一自然日(用户本地时区)内的多笔、以及分属相邻两日的交易
- **THEN** 同一自然日的交易归入同一日期段,不同自然日的交易分属不同日期段,日期段按从新到早排列

### Requirement: widget 资源自包含且不发起外部请求
tally-mcp 托管的每一个 `ui://` widget 资源 SHALL 以 mime 类型 `text/html;profile=mcp-app` 提供,且为单文件自包含:内联所需的 Apps SDK 浏览器运行时、样式与脚本,不引用任何外部脚本、样式表、字体或图片 URL。widget 内所有对账本数据的读取 SHALL 通过 `callServerTool` 走 tally-mcp 自身的工具,不直接发起跨域网络请求。widget SHALL 不做任何写操作。

#### Scenario: 资源可被宿主直接渲染
- **WHEN** 宿主对 widget 的 `ui://` URI 发起 `resources/read`
- **THEN** 返回的内容 mime 类型为 `text/html;profile=mcp-app`,且是一个不依赖任何外部 URL 即可运行的完整 HTML 文档

#### Scenario: widget 只读
- **WHEN** 审视 widget 脚本发起的所有 `callServerTool` 调用
- **THEN** 全部指向只读工具(如 `search_transactions`);没有任何调用会创建、修改或删除数据

### Requirement: 跟随宿主主题
时间线面板 SHALL 读取宿主上下文的主题(浅色/深色)并据此渲染,且 SHALL 订阅主题变化在用户切换时实时更新配色。

#### Scenario: 宿主为深色主题
- **WHEN** 宿主当前为深色主题时打开面板
- **THEN** 面板以深色配色渲染,文本与背景对比度可读

#### Scenario: 打开后切换主题
- **WHEN** 面板已渲染,用户在宿主中把主题从浅色切到深色(或反之)
- **THEN** 面板配色随之更新,无需重新打开
