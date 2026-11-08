## 1. 读目标网页、固定映射

- [x] 1.1 读共享记账导入页的前端脚本(`app.6dc585f9.js`,2026-10 版),确认它实际读取的列与列顺序。验证:结论落到 `plugin/skills/export/SKILL.md` 第 2 节,表头为 `日期,类型,分类,金额,货币,备注,账户`,并记录"多出的列(含子分类)会被忽略"。
- [x] 1.2 确认分类列的匹配方式与日期解析行为。验证:SKILL.md 写明"按一级分类名完全相等匹配"与"解析时 `add(1,"days")`",据此定出"只写一级""日期减一天"两条规则。
- [x] 1.3 确认账户列与备注列的处理方式。验证:SKILL.md 写明账户固定写"默认"(不写 tally 来源名)、备注原样写 `comment` 且超过 60 字要在回复里提示。

## 2. 落地 skill

- [x] 2.1 在 `plugin/skills/export/SKILL.md` 写入导出流程:先划范围、`next_cursor` 翻到底、同范围时间线摘要留作对账。验证:通读第 1 节,三条要求均有对应文字。
- [x] 2.2 写入生成与交付纪律:脚本生成、`utf-8-sig`、`newline=""`、升序、单文件 ≤3000 笔、文件名带范围。验证:第 4 节覆盖全部六点。
- [x] 2.3 写入对账与交付说明:行数、金额合计、排除的收入笔数、日期提前一天的提示;不一致不交付。验证:第 5 节与 spec 的「交付前对账」「日期整体提前一天」逐条对应。
- [x] 2.4 写入纪律:不擅自替换命名;换目标服务先读其前端或做小样测试。验证:第 6 节存在,且与 spec 的「不擅自替换或对齐命名」一致。

## 3. 文档一致性

- [x] 3.1 确认 `plugin/README.md` 的 skill 清单覆盖 export(或将清单改为不再依赖硬编码数量)。验证:`grep -n "three skills" plugin/README.md` 无命中,或该段已列出全部 5 个 skill。

## 4. 校验与归档

- [x] 4.1 `openspec validate add-plugin-export-skill --strict` 通过。验证:命令退出码 0。
- [x] 4.2 归档本变更,生成 `openspec/specs/plugin-export-skill/spec.md`。验证:`openspec list` 无活跃变更,且主 spec 的 Purpose 段非 TBD、Requirements 与 delta 一致。

## Workflow follow-up

- 归档用 `openspec archive add-plugin-export-skill`;归档目录日期取归档当天 `2026-11-08`(`.openspec.yaml` 的 `created` 仍记 skill 实际落地日 `2026-10-19`)。
- 归档后核对 `openspec/specs/plugin-export-skill/spec.md` 的 Purpose 是否原样带入(delta 已含 Purpose,正常无需手改)。
