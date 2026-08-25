## 1. plugin-record-skill:新增"无合适分类时提议新建"

- [x] 1.1 在 `plugin/skills/record/SKILL.md` 的"记录一笔新交易"流程里,先例检索和已有分类都无匹配时,加入预览阶段提议新建分类、确认后调用 `manage_category`(`operation="create"`)的步骤;对照 `plugin-record-skill` spec 里"无合适分类时可在预览中提议新建"的两个 Scenario(提议新建、用户拒绝新建)逐条核对正文是否覆盖
- [x] 1.2 核对 `plugin/skills/record/SKILL.md` 现有正文(先例检索、对话预览、comment 纪律、修正交易前先读、时间解析、keyword 字面匹配、MEMORY.md 判据)是否覆盖 `plugin-record-skill` spec 里对应的每一条 Requirement,补齐遗漏后逐条打勾确认

## 2. plugin-analysis-skill:新建 skill

- [x] 2.1 创建 `plugin/skills/analysis/SKILL.md`,写好触发用的 `description`(覆盖消费分析/对比/趋势相关的自然语言意图,如"这个月花多了没""这家店是不是又涨价了")
- [x] 2.2 在正文里落实四种查询角度——同类交易价格对比、月度环比、类别拆分对比、多月/多年趋势——分别对应 `plugin-analysis-skill` spec 里的四条 Requirement,给出对应的 `search_transactions`/`get_financial_summary` 调用方式说明
- [x] 2.3 在正文里落实三条解读规则——`net` 符号语义、多币种不换算、分类/来源拆分仅含有交易的项——对照 spec 逐条核对覆盖

## 3. plugin-optimize-skill:新建 skill

- [x] 3.1 创建 `plugin/skills/optimize/SKILL.md`,写好触发用的 `description`(覆盖分类拆分/合并/定期复盘相关意图)
- [x] 3.2 落实拆分/合并信号检测的判断标准,对应 spec 的"检测分类拆分信号"/"检测分类合并信号"两条 Requirement
- [x] 3.3 落实合并的安全执行流程(先用 `search_transactions(category_id=A)` 呈现影响范围 → 逐笔 `update_transaction` 重新分类 → 空分类走 `manage_category` delete 的 preview → apply → 交易量大时提前告知成本),对应 spec 三条 Requirement
- [x] 3.4 落实拆分的安全执行流程(先 `manage_category` create → 展示将移动的交易列表/数量 → 逐笔重新分类),对应 spec"拆分同样遵循先呈现后执行"
- [x] 3.5 落实周期性复盘建议的写法——检查当次会话工具/skill 列表里是否有宿主提供的定时能力,有则用、没有则明确告知用户——对应 spec 两条 Scenario;正文里不出现任何具体定时工具名(如 `CronCreate`、`schedule` 等),避免写死会过期的底层 API

## 4. 收尾

- [x] 4.1 运行 `openspec validate --change expand-plugin-practice-skills --strict`,确认三份 spec 与 proposal 通过校验、无格式问题
- [x] 4.2 通读三份 skill 正文,确认 record 的"顺手建分类"与 optimize 的"拆分/合并分类"之间职责边界清楚、没有重叠或矛盾表述(通读发现 optimize 开头"分粗一点没关系"与后文"默认偏细分"自相矛盾,已改为"分细一点没关系")
