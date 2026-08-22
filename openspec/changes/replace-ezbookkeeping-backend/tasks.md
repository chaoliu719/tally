## 1. 数据访问层基础设施

- [x] 1.1 引入 `sqlc` 与 `modernc.org/sqlite` 依赖,添加 `sqlc.yaml` 配置,验证 `sqlc generate` 能跑通(哪怕 schema/queries 暂时是空的)
- [x] 1.2 按 [design.md](design.md) 的 DDL 写 `internal/store/schema.sql`(accounts / categories / transactions 三张表、索引、CHECK 约束),用 `go:embed` 嵌入并在启动时执行,验证:对一个全新的临时 SQLite 文件执行后三张表都存在,重复执行不报错(`CREATE TABLE IF NOT EXISTS` 幂等)
- [x] 1.3 打开数据库连接时设置 `journal_mode=WAL` 与 `busy_timeout`,验证一个测试里查询 `PRAGMA journal_mode` 返回 `wal`

## 2. 币种参考数据

- [x] 2.1 建一份对齐 ISO 4217 标准的静态币种表(代码 → 小数位数),验证:CNY/USD/EUR 等常见币种在表中且为 2 位小数,JPY/KRW 等为 0 位,BHD/KWD/OMR 等为 3 位;在工具的 jsonschema 描述里准确说明"最小单位随币种变化",不再沿用固定两位小数的说法

## 3. sqlc 查询层

- [x] 3.1 写 `internal/store/queries.sql`,覆盖账户增/查、分类增/查、交易增/查/搜索、账户余额聚合(`SUM(amount)`)查询,跑 `sqlc generate` 生成 Go 代码,验证 `go build ./internal/store/...` 通过
- [x] 3.2 为生成的查询层写不经过 MCP 工具层的直接测试:插入交易后聚合余额正确;`CHECK` 约束会拒绝"`income`/`expense` 缺 `category_id`"和"`balance_adjustment` 带 `category_id`"这两种写入

## 4. 账户管理工具

- [x] 4.1 重写 `list_accounts`,余额从 `SUM(amount)` 现算,验证 [account-management/spec.md](../../specs/account-management/spec.md) 的两个既有场景(空账本返回空列表、已有账户返回完整字段)仍然成立
- [x] 4.2 重写 `manage_account` 的创建路径:货币校验换成新的静态表;非零初始余额时,账户行与一笔 `balance_adjustment` 交易行写在同一个 SQL 事务里。验证 spec 里创建成功、缺字段、不支持币种三个场景仍然成立,并额外验证非零初始余额创建后 `SUM(amount)` 算出的余额与指定初始值一致

## 5. 分类管理工具

- [x] 5.1 重写 `list_categories`/`manage_category`,验证 [category-management/spec.md](../../specs/category-management/spec.md) 的全部场景(一级分类创建、二级分类创建、二级分类下不能再建子分类、parent_id 指向不存在的分类、缺字段)仍然成立

## 6. 交易记录工具

- [x] 6.1 重写 `create_transaction`:金额按 design.md 的约定带符号写入(`income` 正、`expense` 负),分类必须是二级分类。验证 [transaction-recording/spec.md](../../specs/transaction-recording/spec.md) 的"提供有效信息记录交易"场景成立,且交易创建后账户余额(现算)按类型正确增减
- [x] 6.2 验证同一 spec 中"引用不存在的账户或分类""引用一级分类""缺少必填字段"三个失败场景:请求被拒绝、不写入交易、余额不变
- [x] 6.3 重写 `get_transaction`,验证已存在交易能查到完整信息(包括 `balance_adjustment` 类型,现在不再被隐藏)、不存在的 ID 返回明确的"未找到"错误
- [x] 6.4 重写 `search_transactions`,去掉现在对 `modify_balance`/`balance_adjustment` 的过滤逻辑。验证 spec 里三个场景(无筛选返回全部、按时间范围筛选、筛选为空返回空列表)仍然成立,并额外验证 `manage_account` 创建时产生的 `balance_adjustment` 交易现在能被搜到

## 7. bootstrap 与依赖清理

- [x] 7.1 删除 `internal/bootstrap/user.go`(整个假用户 bootstrap 逻辑)与 `TALLY_DEFAULT_CURRENCY` 配置项,`internal/bootstrap/datastore.go` 改为用新 schema 初始化,验证启动一个全新的 DB 文件成功、连续启动两次不报错、数据库里不存在用户表
- [x] 7.2 从 `go.mod` 移除 `github.com/mayswind/ezbookkeeping` 依赖并跑 `go mod tidy`,验证 `go build ./...` 全仓库通过,且 `go.sum` 里不再出现 ezbookkeeping 及其专属间接依赖(如 `mattn/go-sqlite3`)

## 8. 端到端验证

- [x] 8.1 从全新的空 SQLite 文件开始,只通过 MCP 工具调用完整走一遍:启动 server → `manage_account` 建一个带非零初始余额的账户 → `manage_category` 建一级分类 → 在其下建二级分类 → `create_transaction` 用该二级分类记一笔支出 → `search_transactions` 既能查到这笔支出、也能查到账户创建时那笔 `balance_adjustment` → `get_transaction` 按支出交易的 ID 查到同一笔交易,且账户余额(现算)已正确扣减,验证 proposal.md 描述的功能对等闭环成立
- [x] 8.2 跑一遍全部单元测试与 `go vet ./...`,并用 `grep -r "mayswind/ezbookkeeping" internal/ cmd/` 确认没有残留引用

## 9. 文档

- [x] 9.1 更新 [README.md](../../../README.md):移除已不存在的 `TALLY_DEFAULT_CURRENCY`,补充"金额最小单位按 ISO 4217 币种变化"的说明,其余环境变量与启动步骤按新实现校对一遍
- [x] 9.2 更新 [openspec/config.yaml](../../config.yaml) 的项目上下文段落,把"把 ezbookkeeping 当 Go 库依赖引入"那段替换成新架构描述(自建 schema、sqlc、无用户概念、无时区处理),内容与这次 change 的实际决策保持一致
