## 1. 工具层改动

- [x] 1.1 `CurrencyTotals.Income`/`Expense`/`Net`、`CategorySummary.Income`/`Expense`、`SourceSummary.Income`/`Expense` 类型从 `int64` 改为 `string`,更新对应字段的 jsonschema 描述,说明其为该分组 `currency` 主单位下的十进制字符串(`net` 可为负)
- [x] 1.2 `getFinancialSummary` 中构造这三个结构体的位置改用 `currency.FormatMajor(row.Currency, ...)` 生成 `income`/`expense`/`net` 字段,`net` 直接传入符号本身(可能为负)的最小单位整数,不做额外取绝对值处理;`go build ./mcp/...` 通过

## 2. 测试更新

- [x] 2.1 改写 `analytics_test.go` 中依赖 `int64` income/expense/net 字面量的既有用例,换成对应的十进制字符串,`go test ./mcp/internal/tools/...` 全部通过
- [x] 2.2 新增至少一个"总支出大于总收入"的用例,验证 `net` 字符串带负号,且 `income`/`expense` 本身仍是非负字符串
- [x] 2.3 新增至少一个非 CNY 币种(JPY 或 BHD)的用例,验证 `get_financial_summary` 返回的 `income`/`expense`/`net` 精度正确,不退化成两位小数

## 3. 全量验证

- [x] 3.1 运行 `go build ./...` 确认整个 `mcp` 模块编译通过
- [x] 3.2 运行 `go vet ./...` 确认无新增告警
- [x] 3.3 运行 `go test ./...` 确认全量测试套件通过
