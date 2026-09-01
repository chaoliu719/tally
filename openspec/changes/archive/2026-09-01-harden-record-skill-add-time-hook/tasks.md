## 1. time-context hook

- [x] 1.1 核对当前 Claude Code plugin 的 hooks 声明约定(目录、`hooks.json` 结构、SessionStart / UserPromptSubmit 事件名),记录在 design.md 的 D3 下,确认与假设一致或据实修正 — 已核对官方文档,与假设一致:`hooks/hooks.json` 自动发现、`plugin.json` 无需声明、路径用 `"${CLAUDE_PLUGIN_ROOT}"`
- [x] 1.2 新增 `plugin/hooks/time-context.sh`:`#!/bin/sh`,一次 `date` 读取,输出一行含「当前日期 + 星期 + 本地时区名与 UTC 偏移」的中文上下文,`exit 0`;加可执行位。手动运行验证输出格式,且 macOS 与 Linux 上均可跑 — `sh` 与直接执行均通过;`%u`/`%Z`/`%z` 为 POSIX 可移植格式;`TZ=` 覆盖测试证明每次现读时钟
- [x] 1.3 新增 `plugin/hooks/hooks.json`(或官方约定的等价配置),把该脚本挂到 SessionStart 与 UserPromptSubmit 两个事件 — JSON 校验通过
- [x] 1.4 如官方约定要求,在 `plugin/.claude-plugin/plugin.json` 声明 hooks 入口 — 官方约定不要求(用 `hooks/hooks.json` 时 `plugin.json` 无需 `hooks` 字段),无需改动
- [ ] 1.5 在本机以插件形式加载,开新会话确认时间上下文被注入;人为改系统日期或等待跨日后提交新输入,确认 UserPromptSubmit 注入的日期已刷新(验证 `plugin-time-context-hook` 的两条注入 Requirement) — 待用户在真实插件环境验证;headless 部分(现读时钟、`${CLAUDE_PLUGIN_ROOT}` 路径形式、输出格式、exit 0)已确认

## 2. record skill 同步

- [x] 2.1 更新 `plugin/skills/record/SKILL.md` 的「时间字段没有时区语义」一节:要求相对/绝对时间一律用 shell `date` 换算、以注入锚点为基准、禁止心算,并在预览回显解析结果。与 `specs/plugin-record-skill/spec.md` 的 MODIFIED 要求逐条对齐
- [x] 2.2 在 `SKILL.md` 新增「跨品类消费提议拆分」一节:触发条件(多品类 / 大额单一商户)、在对话预览中给出按子类拆分的方案、用户可否决、确认后逐笔 `create_transaction`;明确子类分类仍逐笔走先例检索,不引入固定规则表
- [x] 2.3 在 `SKILL.md` 新增「退款冲回原分类」一节:先 `search_transactions` 定位原始支出 → 复用其 `category_id`/`source_id` → 预览标明冲回对象;定位不到则询问,不新建「退款」分类
- [x] 2.4 强化 `SKILL.md` 的「comment 纪律」:写明 tally 无商户字段,`comment` 必须同时含商户原名与商品明细;拆分产生的每笔各自带商户名 + 该笔明细,保证可被 `search_transactions` 单独命中
- [x] 2.5 通读 `SKILL.md`,确认四处改动与 `specs/plugin-record-skill/spec.md` 的全部 Requirement/Scenario 一一对应,无遗漏、无冲突 — 11 条 Requirement 全部对应;并微调「不在范围内」一节措辞,消除与拆分小节的口径冲突

## 3. 收尾

- [x] 3.1 `openspec validate "harden-record-skill-add-time-hook" --strict` 通过
- [ ] 3.2 走一遍端到端手动验证:新会话 → 说「前天在沃尔玛买了 167.8，有食材有日用有零食」→ 确认 agent 用 `date` 把「前天」解析成注入锚点前两天、在预览里回显日期、提议按子类拆分、每笔 comment 带「沃尔玛 + 明细」;确认无先例时才追问分类 — 待用户在真实插件环境跑
