## 1. Store 层查询

- [x] 1.1 修改 `internal/store/queries.sql` 里的 `SearchTransactions`:新增可选的 `keyword`(`sqlc.narg`)过滤条件 `AND (sqlc.narg('keyword') IS NULL OR LOWER(comment) LIKE '%' || LOWER(sqlc.narg('keyword')) || '%' ESCAPE '\')`,不改动现有的 `source_id`/`category_id`/`start_time`/`end_time`/`after_time`/`after_id` 过滤条件与 `ORDER BY time, id`
- [x] 1.2 运行 sqlc 生成代码,确认 `internal/store` 下 `SearchTransactions` 的生成方法签名包含新的 `Keyword` 参数(可空字符串类型),且 `go build ./...` 通过

## 2. LIKE 通配符转义

- [x] 2.1 在 `internal/tools`(建议放进 `transactions.go` 或新文件)实现一个转义函数,把用户提供的 `keyword` 中的 `\`、`%`、`_` 三个字符按顺序(先转义 `\` 本身,再转义 `%`/`_`)转成 `\\`/`\%`/`\_`,供拼进 `LIKE ... ESCAPE '\'` 查询前调用
- [x] 2.2 单元测试覆盖转义函数:普通文本不受影响、包含单个 `%`、单个 `_`、单个 `\`、以 `\` 结尾、同时包含三种字符的组合,逐一验证转义后的字符串按预期转义(不需要跑真实 SQL,纯字符串断言即可)

## 3. cursor fingerprint 接入 keyword

- [x] 3.1 在 `internal/tools/transactions_cursor.go` 的 `searchTransactionsFilterFields` 结构体新增 `Keyword string` 字段(存储 trim 后的规范值,未提供时为空字符串),让它随其余字段一起参与 `searchTransactionsFilterFingerprint` 的 JSON 序列化 + SHA-256
- [x] 3.2 单元测试覆盖:相同筛选条件加相同 `keyword` 得到相同 fingerprint;其余筛选条件不变、仅 `keyword` 不同(含"从有 keyword 变成无 keyword"与"从无变有"两种切换)得到不同 fingerprint;用带 `keyword` 签发的 cursor 在无 `keyword` 的请求中解码应返回「cursor 无效」错误,反之亦然

## 4. `search_transactions` 工具改造

- [x] 4.1 `SearchTransactionsInput` 新增 `Keyword string`(可选,`omitempty`)字段,更新 jsonschema 描述,说明其为大小写不敏感的 `comment` 子串匹配、与其他筛选条件 AND 组合、空白值视为未提供
- [x] 4.2 `searchTransactions` handler:对 `in.Keyword` 做 trim,trim 后为空则视为未提供(不设置查询参数、`filter.Keyword` 保持零值);非空则转义后设置 `params.Keyword`(sql.NullString 或等价可空类型),并把 trim 后的规范值写入 `filter.Keyword` 供 fingerprint 使用
- [x] 4.3 确认 `params.Keyword` 未提供时传递给 sqlc 生成代码的是 SQL `NULL`(与现有 `SourceID`/`CategoryID` 等 `sqlc.narg` 参数的"未提供即 NULL"处理方式一致),而不是空字符串(空字符串会被 `LIKE '%%'` 命中全部行,语义错误)

## 5. 测试

- [x] 5.1 在 `internal/tools/transactions_test.go` 补充测试:提供 `keyword` 且账本内有交易的 `comment` 命中该关键词子串(大小写不同也命中),验证只返回命中的交易
- [x] 5.2 补充「`keyword` 与 `source_id`/`category_id`/`start_time`/`end_time` 组合使用,只返回同时满足全部条件的交易」的测试
- [x] 5.3 补充「`keyword` 包含字面 `%`/`_` 字符,只命中 `comment` 中包含该字面字符的交易,不被当作通配符」的测试
- [x] 5.4 补充「`keyword` 为空字符串或纯空白时按未提供处理,不拒绝请求、不影响返回结果」的测试
- [x] 5.5 补充「`keyword` 未命中任何交易时返回空列表、不返回 `next_cursor`」的测试
- [x] 5.6 补充「用 `keyword` 分页翻页,拼接每页结果等于该 `keyword` 下不分页的全量结果、无重复无遗漏」的测试,验证 keyset 分页在有 `keyword` 过滤时依然正确
- [x] 5.7 补充「用某个 `keyword` 签发的 `next_cursor`,换成不同 `keyword`(含换成空/未提供)重放,请求被拒绝并返回 cursor 无效错误」的测试,对应 spec 中「cursor 无效或已不匹配当前筛选条件」场景的新增部分
- [x] 5.8 运行 `go test ./internal/tools/...` 确认新增测试与现有测试全部通过

## 6. 收尾

- [x] 6.1 运行完整测试套件(`go test ./...`)确认无回归
- [x] 6.2 检查仓库内其他直接调用 `search_transactions` 的测试或文档,确认新增的可选 `keyword` 参数不影响它们现有的调用方式与预期结果(不提供 `keyword` 时行为与变更前完全一致)
