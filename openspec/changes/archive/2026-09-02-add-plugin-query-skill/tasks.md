## 1. plugin-query-skill:新建 skill

- [x] 1.1 创建 `plugin/skills/query/SKILL.md`,写好触发用的 `description`(覆盖「查某一笔/某商户历史」「查一段时间清单」这类只读意图,并写明「要合计/对比 → analysis」的判据)
- [x] 1.2 正文落实检索方式——`search_transactions` 的 keyword 子串/无通配符语义、`get_transaction` 取单笔全量、多账本先 `list_ledgers`——对照 spec「多账本时先确认账本」「检索某商户或关键词的历史交易」「检索一段时间的交易清单」三条 Requirement 逐条核对
- [x] 1.3 正文落实时间换算纪律——查询前用 `date` 算窗口成 Unix 秒、查询后用 `date -r`/`date -d @` 反查返回值再展示,全程禁止心算/脑补偏移——对照 spec「查询前把时间窗口用 shell date 换算成 Unix 秒」「查询后把返回的秒数字段用 shell date 反查再展示」两条 Requirement
- [x] 1.4 正文落实返回值解读三条——`amount` 符号方向不默认支出、多币种不换算、`comment` 原文优先——对照 spec「正确解读返回值的金额、币种与 comment 语义」
- [x] 1.5 正文写明与 `record`/`analysis`/`optimize`/图表的边界,对照 spec「明确 query 与其余 skill 的边界」

## 2. 边界协调

- [x] 2.1 收窄 `plugin/skills/analysis/SKILL.md` 的 `description`,把「原样列明细」指向 `query`;在其「不在这个 skill 范围内」一节补一句指向 `query`/`record`
- [x] 2.2 通读四份 skill 正文,确认 `query`(只读)与 `record`(增改)、`analysis`(合计/对比)、`optimize`(分类结构)之间职责边界清楚、无重叠或矛盾表述

## 3. 收尾

- [x] 3.1 运行 `openspec validate add-plugin-query-skill --strict`,确认 spec 与 proposal 通过校验
- [x] 3.2 archive:`openspec archive add-plugin-query-skill`
