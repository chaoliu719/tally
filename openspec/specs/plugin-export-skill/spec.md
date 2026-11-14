# plugin-export-skill Specification

## Purpose
定义 agent 把账本中已经存在的交易导出为第三方账单导入页可直接消费的 CSV 时的规范:目标格式与列映射、日期与账户等硬性取值、文件生成方式,以及交付前的对账。这是纯只读能力——导出不改变 tally 里的任何数据,也不负责把外部账单导入 tally。

## Requirements

### Requirement: 导出是只读操作
导出 SHALL 只使用读类工具(`search_transactions`、`list_categories`、`list_sources`、`list_budgets`、`list_ledgers`、`open_transaction_timeline`)读取既有交易,SHALL NOT 新增、修改或删除 tally 中的任何交易、分类、来源或预算。

#### Scenario: 导出过程中发现分类需要调整
- **WHEN** agent 认为某笔交易的分类名不适配目标服务,导出时会更容易导入
- **THEN** agent 不修改该交易,按 skill 规定的映射原样导出,并在回复里说明差异

### Requirement: 导出前划定范围并取回全量
Agent SHALL 在导出前把用户给出的范围换算成明确的时间窗口(相对表达用 shell `date` 计算,不心算),SHALL 用 `next_cursor` 翻页直到取全该范围内的所有交易,SHALL NOT 只导出第一页。存在多个账本且用户未指明时,Agent SHALL 先用 `list_ledgers` 让用户选择。

#### Scenario: 用户要某个月的账单
- **WHEN** 用户说"导出十月的账"
- **THEN** agent 以 10-01 00:00:00 到 10-31 23:59:59 为范围翻页取全交易,并记录该范围内的笔数与支出合计

#### Scenario: 结果超过一页
- **WHEN** `search_transactions` 返回的 `next_cursor` 不为空
- **THEN** agent 用该游标继续翻页,直到没有更多结果,不把第一页当作全量

### Requirement: CSV 表头固定为共享记账认的 7 列
导出的 CSV SHALL 使用固定表头,顺序固定:`日期,类型,分类,金额,货币,备注,账户`,SHALL NOT 增加"子分类"或其他列——目标网页只读这 7 列,多余列会被忽略。

#### Scenario: 用户要求带上二级分类
- **WHEN** 用户希望导出的表格里保留二级分类信息
- **THEN** agent 说明目标网页没有子分类列、多出的列会被忽略,仍按 7 列导出,并说明二级分类需导入后在网页里逐条调整

### Requirement: 类型、金额与货币按目标网页语义取值
`类型` SHALL 只能是"支出"或"收入"。`金额` SHALL 写正数并保留原值精度。`货币` SHALL 写"人民币"——目标网页会清空该列,取值不影响导入。用户未明确要求收入时,Agent SHALL 只导出支出;排除了收入类交易时 SHALL 在回复里说明排除了几笔。

#### Scenario: 范围内含利息收入
- **WHEN** 用户要导出某段时间的支出账单,范围内存在一笔利息收入
- **THEN** agent 不导出该笔,并在交付说明里写明"另有 1 笔收入未导出"

### Requirement: 分类只写一级分类名
`分类` SHALL 写交易所属分类的**一级分类名**:若该分类的 `parent_id` 不为 `"0"`,取父分类名;否则取自身名称。Agent SHALL NOT 输出"一级-二级"或"一级/二级"之类的拼接,因为目标网页按一级分类名做完全相等匹配,拼接值一定匹配不上。

#### Scenario: 交易的分类是二级分类
- **WHEN** 一笔交易的 `category_id` 指向"餐饮/外卖"这样的二级分类
- **THEN** agent 用 `list_categories` 查到其父分类并写"餐饮",不写"餐饮-外卖"

### Requirement: 备注原样保留
`备注` SHALL 直接写交易的 `comment` 原文,SHALL NOT 改写、精简或补充解释。`merchant` SHALL NOT 被拼接到 `备注` 列,也 SHALL NOT 为它新增列——目标网页只认固定的 7 列,商户名不进入导出文件。`comment` 超过 60 字时,Agent SHALL 在回复里提示目标网页的备注输入框有长度限制、预览中可能被截断。

#### Scenario: 备注含账单原文
- **WHEN** 交易的 `comment` 是"美团-外卖订单 1234,含配送费"
- **THEN** CSV 里该字段与 `comment` 完全一致,不做任何加工

#### Scenario: 交易同时有商户名与备注
- **WHEN** 一笔交易的 `merchant` 是"星巴克(南京西路店)"、`comment` 是"和同事下午茶"
- **THEN** CSV 的 `备注` 列只写"和同事下午茶",不写成"星巴克(南京西路店) 和同事下午茶",也不新增商户列

#### Scenario: 交易只有商户名没有备注
- **WHEN** 一笔交易的 `merchant` 有值、`comment` 为空
- **THEN** CSV 的 `备注` 列为空,不把 `merchant` 填进该列

### Requirement: 账户列固定写默认
`账户` SHALL 统一写"默认"。Agent SHALL NOT 把 tally 的来源名(如"信用卡(尾号3473)")写进该列,因为目标网页按账户名完全匹配,匹配不上会显示 `-1`。

#### Scenario: 交易来源是信用卡
- **WHEN** 一笔交易的 source 名为"信用卡(尾号3473)"
- **THEN** CSV 的 `账户` 列仍写"默认",不写来源名

### Requirement: 日期整体提前一天
由于目标网页在解析日期时固定加一天,CSV 中的日期 SHALL 比账本中的交易时间早一天,格式为 `YYYY-MM-DD HH:MM:SS`,时分秒保留。月初交易落到上个月最后一天属于预期结果。Agent SHALL 在交付说明里讲明"文件里的日期比账本早一天,是目标网页的解析行为",避免用户误以为导错。

#### Scenario: 导出月初的交易
- **WHEN** 交易时间为 `2026-10-01 08:42:00`
- **THEN** CSV 中写 `2026-09-30 08:42:00`,并在交付说明里提示日期已提前一天

### Requirement: 用脚本生成文件
Agent SHALL 用脚本(而非手工拼接文本)生成 CSV,使用 `utf-8-sig` 编码(带 BOM)与 `newline=""`,按时间升序排列,文件名包含范围与用途。单个文件 SHALL NOT 超过 3000 笔,超过时按月拆分。

#### Scenario: 范围超过 3000 笔
- **WHEN** 某次导出范围内的交易超过 3000 笔
- **THEN** agent 按月拆成多个文件,每个文件不超过 3000 笔,并说明拆分方式

### Requirement: 交付前对账
交付前 Agent SHALL 用同范围的 `open_transaction_timeline` 摘要核对两件事:CSV 行数等于范围内笔数(减去被排除的收入笔数),金额合计等于摘要里的支出合计。不一致时 SHALL NOT 交付,先查原因。交付说明 SHALL 包含笔数、合计、被排除的笔、以及日期提前一天这一提示。

#### Scenario: 行数与摘要不符
- **WHEN** 生成的 CSV 行数与时间线摘要的笔数不一致
- **THEN** agent 不交付文件,先定位是分页没取全、分类映射丢行还是过滤条件写错,修好并重新对账

### Requirement: 不擅自替换或对齐命名
Agent SHALL NOT 未经用户确认改写分类名、备注、来源或日期取值方式。目标服务需要不同格式时,Agent SHALL 先确认它认什么列与取值(必要时读其前端脚本或先做单条小样测试),再出文件。

#### Scenario: 用户要求换一个目标服务
- **WHEN** 用户要导出的目标不再是共享记账,而是另一个导入页面
- **THEN** agent 先确认该服务认的列与取值格式,必要时先导一条做小样验证,不沿用旧映射直接出全量文件

### Requirement: 导出不携带预算
导出的 CSV SHALL 保持目标导入页认的固定列,不新增预算列,也不把预算名称写入分类列或其他任何列。预算信息在导出时丢失是预期行为,Agent SHALL 在交付时向用户说明这一点。导出 SHALL NOT 为了让预算信息得以保留而改写任何交易的分类或备注。

#### Scenario: 导出一段包含多个预算的交易
- **WHEN** 用户要求导出一段时间的交易,这些交易分属不同预算
- **THEN** 生成的 CSV 仍是目标导入页认的固定列,不含预算信息,agent 在交付时说明预算没有被导出

#### Scenario: 想用分类列保留预算
- **WHEN** agent 考虑把预算名称拼进分类列,以免导出后丢失预算信息
- **THEN** agent 不这样做,分类列仍只写一级分类名,预算信息留在账本内不进入导出文件
