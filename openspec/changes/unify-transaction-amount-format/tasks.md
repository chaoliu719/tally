## 1. 币种换算函数

- [x] 1.1 在 `mcp/internal/currency` 新增 `FormatMajor(code string, minorUnits int64) (string, error)` 与 `ParseMajor(code string, s string) (minorUnits int64, err error)`,仅用整数/字符串运算实现(不经过 `float64`/`math/big.Float`),`go build ./mcp/...` 通过
- [x] 1.2 为 `FormatMajor`/`ParseMajor` 编写单元测试,覆盖 CNY(2 位小数)、JPY(0 位小数)、BHD(3 位小数)三种精度的格式化与解析,以及非法输入(非数字字符、多个小数点、空字符串、精度超限、零或负值),`go test ./mcp/internal/currency/...` 全部通过

## 2. 工具层改动

- [x] 2.1 `CreateTransactionInput.Amount`/`UpdateTransactionInput.Amount` 类型从 `int64` 改为 `string`,更新对应字段的 jsonschema 描述,说明其为该 `currency` 主单位下的十进制字符串
- [x] 2.2 `TransactionInfo.Amount` 类型从 `int64` 改为 `string`,更新 jsonschema 描述;`toTransactionInfo` 改用 `currency.FormatMajor(t.Currency, t.Amount)` 生成该字段
- [x] 2.3 调整 `validateTransactionInput` 的校验顺序为"先校验 `currency` 受支持,再用 `currency.ParseMajor` 解析并校验 `amount`"(格式合法、精度不超过该币种标准位数、解析后为正),各类失败返回与现有风格一致的明确错误
- [x] 2.4 确认 `signedAmount`/`abs64` 相关的正负号逻辑在改用 `ParseMajor` 解析出的整数后行为不变(income 存正、expense 存负,读出时按类型换算,不受这次改动影响),`go build ./mcp/...` 通过

## 3. 测试更新

- [x] 3.1 改写 `transactions_test.go` 中依赖 `int64` amount 字面量的既有用例,换成对应的十进制字符串,同时保留对底层 `store` 层最小单位整数值的断言(两层互相验证,不能只断言 wire 字符串),`go test ./mcp/internal/tools/...` 全部通过
- [x] 3.2 为 `create_transaction`/`update_transaction` 新增失败路径用例:金额格式非法、精度超出该币种标准位数、金额为零或负值,验证请求被拒绝且交易未被创建/未发生变化
- [x] 3.3 新增至少一个非 CNY 币种(JPY 或 BHD)的端到端用例,验证 `create_transaction` → `get_transaction` → `search_transactions` 全链路的 `amount` 字符串精度正确,不退化成两位小数

## 4. 全量验证

- [x] 4.1 运行 `go build ./...` 确认整个 `mcp` 模块编译通过
- [x] 4.2 运行 `go vet ./...` 确认无新增告警
- [x] 4.3 运行 `go test ./...` 确认全量测试套件通过
