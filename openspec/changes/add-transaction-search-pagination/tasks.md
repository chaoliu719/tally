## 1. Store 层查询

- [ ] 1.1 修改 `internal/store/queries.sql` 里的 `SearchTransactions`:新增可选的 `after_time`/`after_id` keyset 过滤(`(time, id) > (?, ?)`,为空时不过滤)与 `LIMIT`,不改动现有的 `account_id`/`category_id`/`start_time`/`end_time` 过滤条件与 `ORDER BY time, id`
- [ ] 1.2 运行 sqlc 生成代码,确认 `internal/store` 下 `SearchTransactions` 的生成方法签名包含新参数,且 `go build ./...` 通过

## 2. 分页 cursor 编解码

- [ ] 2.1 在 `internal/tools/transactions.go`(或同包新文件)实现 cursor 的编码/解码:内容为 `{last_time, last_id, filter_fingerprint}`,`filter_fingerprint` 对 `account_id`/`category_id`/`start_time`/`end_time` 序列化后取 SHA-256(参照 [accounts.go](internal/tools/accounts.go) 的 `accountDeletionRevision` 写法,但不引入签名密钥),外层用 base64url 编码,解析失败或 fingerprint 不匹配时返回明确错误
- [ ] 2.2 单元测试覆盖:合法 cursor 往返编解码一致、cursor 内容损坏时返回错误、cursor 是用不同筛选条件签发时返回错误(对应 spec 中「cursor 无效或已不匹配当前筛选条件」场景)

## 3. `search_transactions` 工具改造

- [ ] 3.1 `SearchTransactionsInput` 新增 `Limit`(可选,默认 50,大于 200 时拒绝请求)与 `Cursor`(可选字符串)字段,更新 jsonschema 描述
- [ ] 3.2 `SearchTransactionsOutput` 新增可选 `NextCursor` 字段,jsonschema 说明"存在更多结果时给出,原样传回 cursor 入参取下一页"
- [ ] 3.3 `searchTransactions` handler:解码 `cursor`(若提供)得到 `after_time`/`after_id`,查询时 `LIMIT` 传 `limit+1`,若结果条数为 `limit+1` 则丢弃最后一条并用第 `limit` 条构造 `NextCursor`,否则不返回 `NextCursor`
- [ ] 3.4 `limit` 未提供时使用默认值 50;`limit <= 0` 或 `limit > 200` 时拒绝请求并返回明确错误

## 4. 测试

- [ ] 4.1 在 `internal/tools/transactions_test.go` 补充测试:无筛选条件时默认按 50 条分页(可用较小的测试专用场景验证跨页而非真造 50+ 笔,比如构造超过默认 limit 的交易,或验证 handler 对自定义小 `limit` 的分页行为),验证 `go test ./internal/tools/...` 通过
- [ ] 4.2 补充「结果超过一页返回 next_cursor,不超过一页不返回」的测试
- [ ] 4.3 补充「用 next_cursor 连续翻页,拼接每页结果等于不分页时的全量结果、无重复无遗漏」的测试
- [ ] 4.4 补充「cursor 无效」「limit 超过 200」两个错误路径的测试
- [ ] 4.5 补充「翻页途中账本发生变化(插入一笔更早/更晚时间的新交易、更新已返回的某笔交易)后继续翻页,不重复不遗漏尚未返回的交易」的测试,对应 spec 的对应场景

## 5. 收尾

- [ ] 5.1 运行完整测试套件(`go test ./...`)确认无回归,包括其他工具对 `search_transactions` 无筛选调用方式的现有测试是否因为默认分页而需要更新
- [ ] 5.2 检查仓库内其他直接调用 `search_transactions`(不带 limit/cursor)的测试或文档,确认新默认行为(每页 50 条而非全量)不会让它们产生误判
