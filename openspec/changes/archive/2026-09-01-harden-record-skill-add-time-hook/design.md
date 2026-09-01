## Context

见 proposal.md - Why。两点约束塑造了本设计:

- `plugin/` 目前只有 skills,没有任何 hook。本 change 引入 `plugin/` 下第一个 hook 组件,
  需要确定目录布局与声明方式。
- 项目对 hook 的既有立场是「实践的强制执行:只执法机械可判定的规则……克制用量,优先
  PreToolUse 精准拦截」。本 change 的 hook 是**上下文注入**而非拦截,属于对该立场的一次
  有意扩展——注入的内容仍是机械可判定的(一次 `date` 读取),不做任何语义判断。

## Goals / Non-Goals

**Goals:**
- record skill 的时间解析、拆分、退款、comment 四条纪律在 spec 与 `SKILL.md` 两处一致。
- 提供一个零配置、无驻留、跨午夜正确的当前时间锚点。

**Non-Goals:**
- 不做时区探测/地理定位;时区取宿主系统本地时区即可。
- 不用 hook 强制执行时间解析纪律(语义判断留给 skill/对话)。
- 不触碰 mcp/ 服务端;`time` 仍是无时区语义的 Unix 秒数。
- 不建分类口径文件(决策 A 已定,放弃)。

## Decisions

### D1: hook 用 SessionStart + UserPromptSubmit 两个触发点

- **SessionStart** 保证会话一开始 agent 就有锚点。
- **UserPromptSubmit** 保证长会话跨过午夜后锚点会刷新。
- 备选:只用 SessionStart —— 被否,长会话跨天后日期会静默过期,正是要解决的 bug 之一。
- 备选:只用 UserPromptSubmit —— 被否,会话首个 assistant 动作若发生在用户输入前(少见)会缺锚点;两个都挂成本极低。

### D2: hook 脚本是一行 shell,输出纯文本上下文

形如 `记账时间锚点:今天 2026-09-01 星期一,本地时区 CST +0800`。
- 取值只用 `date`,不读任何缓存或环境变量覆盖。
- 输出一行、`exit 0`、不驻留。
- 备选:输出 JSON / 结构化块 —— 无必要,agent 读一行自然语言即可。

### D3: hook 放 `plugin/hooks/`,配置用 plugin 约定的 `hooks/hooks.json`

已核对官方 plugin 文档(code.claude.com/docs/en/plugins-reference),与假设一致:

- 位置:插件根下的 `hooks/hooks.json` 会被自动发现,`plugin.json` **不需要**再声明
  `hooks` 字段(内联声明是另一条可选路径,本 change 不用)。
- 结构与用户级 hooks 相同:`{ "hooks": { "SessionStart": [ { "hooks": [ { "type":
  "command", "command": "..." } ] } ], "UserPromptSubmit": [ ... ] } }`。
- 脚本路径用 `"${CLAUDE_PLUGIN_ROOT}"/hooks/time-context.sh`(官方要求给路径变量套
  双引号)。
- 目录:`plugin/hooks/time-context.sh` + `plugin/hooks/hooks.json`。
- 脚本加可执行位;用 `#!/bin/sh` 保持可移植(macOS + Linux 都要跑)。
- SessionStart / UserPromptSubmit 的 command hook,stdout(exit 0)会被作为上下文
  追加进对话,正是所需行为。

### D4: record skill 的拆分只到「提议 + 预览」为止

- skill 教 agent「何时提议拆分、如何在预览里展示」,不规定具体子类怎么分——子类分类
  仍逐笔走先例检索 + 现有分类。
- 这样拆分逻辑不沉淀成一张会漂移的规则表,与决策 A 一致。

### D5: 退款方向复用 tally 既有数据模型

- 不新增字段、不新增「退款」语义;退款就是一笔归属原分类/来源的交易,金额方向按
  现有 amount 符号约定表示。实现阶段以 `create_transaction` 现有校验行为为准。

## Risks / Trade-offs

- **[hook 立场扩张]** 本 change 让 hook 从「只拦截」扩展到「也注入上下文」→ 在 design 与
  proposal 里显式记录这次扩展的理由(注入内容机械可判定、无语义判断、无驻留),供后续
  hook 设计参照,避免被当作「hook 可以随意做任何事」的先例。
- **[SKILL.md 与 spec 漂移]** 四条纪律同时存在于 spec 和 `SKILL.md` → 实现任务里把「同步
  SKILL.md」列为独立可勾选项,archive 时对照 spec 复核。
- **[hooks.json 格式未核实]** 官方 plugin hooks 声明格式可能与假设不符 → D3 标注实现阶段
  先核对官方约定,不硬编造格式。
- **[相对日期算术仍靠 agent]** hook 只给锚点,「前天=锚点-2」「上周五」的推算仍是 agent
  行为 → 由 skill 要求用 `date -d` 计算 + 预览回显兜底,不追求 100% 消除。
