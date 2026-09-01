## Why

记忆分层里沉淀了一批「属于实践层、却没进 skill」的记账规则,agent 每次会话重复踩坑:
把用户说的「前天」凭印象心算成错误日期、逛超市一张小票塞成一笔糊账、退款不知道往哪
记、拆出来的多笔互相搜不到。其中「现在是哪一天、在哪个时区」这个锚点问题,skill 写多少
遍都没用——它是运行时事实,只能由 host 在每轮对话注入。这个 change 把可固化的纪律写进
record skill,并新增插件的第一个 hook 来供给时间锚点。

## What Changes

- **record skill 硬化(修改 `plugin-record-skill` spec)**
  - 自然语言时间 → Unix 秒数的换算,必须走 shell `date`(如 `date -d @<ts>` /
    `date -d "<expr>"`),禁止心算或凭训练知识/对话上下文推断;相对表达一律以 hook
    注入的「今天」为锚点计算。
  - 一笔跨多品类的商户消费(典型:超市),agent SHALL 在对话预览里提议按商品子类拆成
    多笔;是否拆分、如何拆分由用户在预览阶段确认,agent 不自行合并成一个笼统分类。
  - 退款 SHALL 冲回原支出所属的分类/来源(通过先例检索定位原交易),而不是新建「退款」
    类目;退款金额方向按 tally 既有数据模型表示。
  - comment 纪律补充为硬约束:tally 无独立商户字段,`comment` SHALL 同时承载商户原名
    与商品明细;拆分产生的每一笔都要各自带足以被 `search_transactions` 单独命中的
    商户名与明细。
- **新增 `plugin-time-context-hook`(插件第一个 hook)**
  - 一个 `SessionStart` + `UserPromptSubmit` hook,向对话注入当前日期、星期与本地时区
    (`date` 输出),让 agent 有可靠的「今天」锚点做相对日期算术。
  - 纯上下文注入,不做任何拦截或语义判断;不引入常驻进程。
- 明确**不做**:分类口径参考文件(`分类口径.md`)——决策已定,放弃;避免在 skill 层
  重建一张会和先例检索冲突、且随分类树漂移的映射表。

## Capabilities

### New Capabilities
- `plugin-time-context-hook`: 插件 hook,在会话开始与每轮用户输入时把当前日期/星期/本地
  时区注入对话上下文,作为 agent 解析相对时间表达的锚点。

### Modified Capabilities
- `plugin-record-skill`: 时间字段解析要求收紧为「必须走 shell、以注入锚点为基准」;
  新增跨品类消费拆分提议、退款冲回原分类、comment 同时承载商户名与明细三条要求。

## Impact

- `openspec/specs/plugin-record-skill/spec.md`:修改若干 Requirement。
- `openspec/specs/plugin-time-context-hook/spec.md`:新建。
- `plugin/skills/record/SKILL.md`:同步 spec 变更(实现阶段)。
- `plugin/hooks/`:新目录,新增 hook 脚本 + `plugin/hooks/hooks.json`(或等价配置);
  这是 `plugin/` 下第一个 hook 组件。
- `plugin/.claude-plugin/plugin.json`:如需声明 hooks 入口则更新。
- 无 mcp/ 服务端改动;无破坏性变更。
