## Why

账本里的数据要能搬出去:用户可能因为换工具、做备份、或要把某段时间的账交给别的记账服务而需要一份表格。目前 `plugin/` 有查账、分析、记录、分类整理四个 skill,唯独没有"把已有交易导出"这一环,Agent 遇到"给我十月的 csv"只能临时拼,把共享记账导入网页的硬性格式约束(列名、一级分类、日期偏移)全压在对话上下文里。

这个 skill 已经在 `4f12498` 直接落地并发布,本变更在事后补记它的规格与决定依据,让 `openspec/specs/` 与实践层保持一致。

## What Changes

- 新增 `plugin-export-skill` 能力,定义"把 tally 里已有交易导出成目标服务能导入的 CSV"这一行为的规范:只读、只处理已存在的交易,不新增/修改/删除任何数据。
- 明确目标格式是"共享记账"账单导入网页的 7 列(`日期,类型,分类,金额,货币,备注,账户`),分类只写一级、日期整体提前一天、账户固定写"默认"、备注原样保留。
- 明确生成与交付纪律:脚本生成 `utf-8-sig` CSV、按时间升序、单文件不超过 3000 笔,交付前用同范围的时间线摘要核对笔数与金额合计。
- 明确边界:明细浏览归 `plugin-query-skill`,分类结构与改账归 `plugin-record-skill` / `plugin-optimize-skill`;本 skill 不做导入,也不擅自"对齐"目标服务的命名。

## Capabilities

### New Capabilities
- `plugin-export-skill`: 把账本里已有交易导出成第三方导入页认的 CSV——目标格式、列与取值映射、日期偏移、文件生成与交付前对账。

### Modified Capabilities
（无）

## Impact

- `plugin/skills/export/SKILL.md`(已存在,本变更只补规格,不改 skill 内容)
- `openspec/specs/plugin-export-skill/spec.md`(归档时生成)
