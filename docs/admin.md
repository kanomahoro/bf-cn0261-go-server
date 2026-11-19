# 管理后台

一个跑在同进程里的小型 HTTP 控制台：Go 侧只读内存态与数据库，写操作走
`game.Server` 的方法或单条 SQL，**不改数据库结构、不改游戏协议**。

## 开启

**后台默认是关闭的**（仓库里的 `config/server.json` 带的是 `"enabled": false` 和空 token）。
要用的话，把这一段改成：

```json
"admin": {
    "enabled": true,
    "listenAddress": "127.0.0.1",
    "listenPort": 26110,
    "token": "换成你自己的长随机串"
}
```

- `enabled` 为 `true` 时启动时多监听一个端口；`false` 时完全不监听
- **`enabled` 为 `true` 时 `token` 必须自己填**：至少 16 个字符、至少 8 个不同字符、
  不能是 `changeme` / `1234567890123456` 这类占位值，否则**服务端直接拒绝启动**
  （`admin.token must be at least 16 characters`）。仓里默认的 token 是空字符串，
  就是为了防止“开箱即开”地被扫到
- 生成一个：`openssl rand -hex 32`（或用面板的随机密码器）
- 连续 10 次 token 错误会使该地址被挡 1 分钟（HTTP 429）
- **默认只绑 `127.0.0.1`**。要从别的机器访问，用 SSH 隧道：
  `ssh -L 26110:127.0.0.1:26110 user@server`，然后把 `listenAddress` 保持不动
- 想直接对内网开放，把 `listenAddress` 改成内网 IP，并确保 `token` 够长
- 用 `bfserver-install.sh` 装完时会再提示一次这一段（默认关闭、怎么开、SSH 隧道怎么建），
  不读文档也不会漏掉

启动成功后，控制台会打印一段提示（英文，便于在日志中一眼看到）：

```
--------------------------------------------------------------
 Admin console : http://127.0.0.1:26110/
 Token         : admin.token in config/server.json
--------------------------------------------------------------
```

这段提示是在**端口绑定成功之后**才打印的，所以看到它就意味着后台真的在监听；
如果**完全没看到这段**，先确认 `admin.enabled` 是 `true`；如果只看到一条
`[Admin] console stopped` 错误日志，就是端口被占用——**游戏服务端照常运行**。

浏览器打开这个地址，第一次会让你输入 `token`（存在浏览器本地）。

## 能做什么

| 页面 | 内容 |
| --- | --- |
| 概览 | 在线玩家数、房间数、中继连接数、运行时长、数据库延迟、版本与构建提交 |
| 在线玩家 | 昵称、**角色 ID**（可点复制）、账号、等级、位置（广场 / 房号+座位）、IP、在线时长；**踢下线 / 封号**；可翻页 |
| 房间 | 房号、标题、模式、地图、**人数（已占 / 可用座位）**、**房主昵称**（悬停看座位号）、阶段、**是否设密码**；**强制解散**；可翻页 |
| 日志 | 内存里最近 5000 条，按关键字过滤（`[Room]`、`[Relay]`、`[Admin]`…），可翻页回看；自动刷新时只看最新一页 |
| 查玩家 | 按账号名 / 昵称 / 账号 ID / 角色 ID 搜索，显示**角色 ID**、等级、金币、点券、封禁状态，结果可翻页；下方列出**已封禁账号**（可翻页），可直接解封 |
| 战绩 | 左表右详情：点任意一行，右侧直接展开泡封/击爆/营救/经验/金币/等级，不用滚到页底；可翻页、可导出 CSV |
| 操作 | 全服公告；改金币 / 点券（填**角色 ID**，先用「核实」看昵称确认） |
| 配置 | 在线编辑一部分配置项，保存前校验；保存后需要**手动重启**才生效 |

「人数」的分母是**可用座位**：总座位减去房主关掉的座位（客户端房间列表里看到的就是这个数），
所以十二人房里关了两个座位会显示成 `1/10` 而不是 `1/12`。

## 模式名

房间列表、战绩列表和 CSV 导出都用同一张模式表命名；表里没有的号会显示成「模式 N」，
所以在加模式时必须同步三处（`internal/admin/modes.go`、`index.html` 的 `MODE` 与战绩筛选下拉）。

| 编号 | 名称 | 编号 | 名称 |
| --- | --- | --- | --- |
| 1 | 竞技 | 14 | 热血团队 |
| 3 | 道具 | 16 | 僵尸BOSS |
| 8 | 僵尸 | 18 | 热血狙击 |
| 12 | 狙击 | 19 | 爆破 |
| 13 | 热血个人 | 20 | 队长战 |
| | | 101 | 海盗 |

`5`/`6`（夺星）只在表里留了名字，用于读旧库记录——本客户端没有这两个模式可选。
加模式或改名时三处必须一起动（`modes.go`、`index.html` 的 `MODE`、战绩筛选下拉）；
目前没有测试守这处一致，改完搜一遍旧名字。

CSV 列：`id, completed_at, mode, mode_name, map, first_score, second_score, winner, end_reason`
（`mode` 是给脚本用的数字，`mode_name` 是给人看的名字）。

**胜方列的翻译**：页面按每个模式自己的约定读 `winner`，CSV 里仍然是数字。

| 模式 | 页面显示 | 库里的 `winner` |
| --- | --- | --- |
| 101 海盗 / 16 僵尸BOSS | 通关 / 失败 | `2` = 通关（座位方），`1` = 失败（对面） |
| 8 僵尸 | 人类 / 僵尸 / 平 | `1` = 人类，`2` = 僵尸（入库时把 team 14 归并成 2），`0` = 平 |
| 13 热血个人 | 个人战 | 通常 `0`：个人战没有队伍总分，真正的胜者是先到击杀上限的座位，记录里表达不了；有人中途离场时会写成 `1`/`2`，页面一律显示「个人战」 |
| 其余（1/3/12/14/18/19/20） | 红 / 蓝 / 平 | `1` = 红、`2` = 蓝、`0` = 平 |

（来源：`internal/game/rooms.go` 的 `pirateWinner` / `pveWinner` / `zombieWinner` 注释与
`store.StoredWinner`。两个合作模式**同向**：都用 `2` 表示座位方通关——海盗的座位在 team 2
（`pirateSeatTeam`），raid 里玩家的人类阵营也是 team 2（`pveHumanTeam`），所以不能按「海盗 `1`」读。）

## 在线配置

「配置」页只开放**一部分**字段：端口、连接上限、空闲超时、中继对外 IP、僵尸等待、日志级别、注册邀请码与限流。

- **保存前会校验**：服务端把合并后的内容写到同目录的 `server.json.new`，用同一套 `config.Load` 校验，
  通过后才备份 `server.json.bak` 并原子替换；任何一步失败都不会动原文件
- **不在线编辑**的：数据库连接与密码、证书路径、文件路径、管理后台自己的端口与 token
- **保存后不会自动生效**：页面弹窗提示需要手动重启服务端
- 没有「重启」按钮，重启请用服务管理器（systemd / NSSM / 手动）

## 自动刷新与翻页

- **每 5 秒自动刷新**：概览、在线玩家、房间、日志（日志需勾选「自动刷新」）。
  战绩、查玩家、已封禁、配置**不会自己变**，只在你操作时拉取
- 六个列表都可翻页。页码是**位置**不是游标，所以列表变短（解封/退房/下线）时
  会自动退回最后一页，不会停在空白页
- **日志翻页会关掉「自动刷新」**：日志是实时尾巴，不关的话窗口每 5 秒又滑回最新；
  翻回第一页再勾上就行
- 每页条数：战绩 10~200、日志 10~500，其余固定 20

## 三个要知道的行为

**封号**：写 `accounts.disabled`、作废该账号尚未使用的登录票据，并立刻断开在线会话。
封号只影响新的登录，已经在对局里的会话会被断开。

**对着已经消失的行操作**（玩家刚下线、房间刚被解散、账号刚被删）：踢人 / 封号 / 解散 / 写钱包
都按**数字 id** 定位，服务端会先校验目标是否存在，不存在就回 `404` / `400` 并在右下角提示原因；
**不会写错对象**。界面无论成败都会重载列表，所以失败的那一行会立刻消失，不会留一行幽灵。

**改金币 / 点券**：只执行一条 `UPDATE character_stats`，**不刷新内存、不下发属性包**。
玩家重新登录、在商店买一次东西、或打完一局结算后才会看到新数值。**金币与点券各按 1 亿上限**
（`store.MaxLucci` / `store.MaxCash`），超了服务端直接拒绝——数据库列本身还能装更多，但运营界面不放开。

要填的是**角色 ID**（数字），不是昵称也不是账号名。界面强制先「核实」：核实前「写入钱包」是禁用的，
并且两个金额框会用当前值填好，所以不会因为漏填一格把另一种货币写成 0。
从「查玩家」点「改钱包」进来时会自动带上该角色并完成核实。

## API（需要 `Authorization: Bearer <token>`）

```
GET  /api/overview             GET  /api/players?limit=&offset=   POST /api/players/kick
POST /api/players/ban          GET  /api/rooms?limit=&offset=     POST /api/rooms/disband
GET  /api/logs?limit=&offset=&q=   GET  /api/search?q=&limit=&offset=
GET  /api/results?mode=&limit=&offset=   GET  /api/results/detail?id=
GET  /api/bans?limit=&offset=  GET  /api/config        POST /api/config
GET  /api/results.csv?mode=
POST /api/balance              POST /api/notice         GET  /api/server
```

六个列表接口（`players` / `rooms` / `logs` / `search` / `bans` / `results`）统一返回
`{"total": 总数, "offset": 实际起点, "limit": 实际每页条数, "items": [...]}`，控制台的分页器直接
拿 `total` 算页数；`offset` 越过末尾时会被拉回最后一页。

`GET /api/server` 返回版本、构建提交、Go 版本、连接池、配置路径与**数据库结构信息**：

```json
{"version":"1.1.0","schema":{"Revision":1,"Note":"baseline 2026-11-17","Applied":"2026-11-17T02:00:01Z"}, …}
```

`schema` 只是标签（当前基线批次），不参与启动校验——服务端比对的是表/列结构（见
[`database/README.md`](../database/README.md)）。控制台「服务端」卡片里会显示这一行。

CSV 下载也是一次普通的 `fetch`，同样带 `Authorization: Bearer <token>` 请求头。
**token 不接受 URL 参数**（会进浏览器历史、日志和任何被转发的链接）。

## 审计

每个写操作都会在日志里留一行，前缀 `[Admin]`：

```
[Admin] kick account=42
[Admin] account ban changed account=42 disabled=true ticketsRevoked=1 kicked=true
[Admin] balance written character=7 lucci=10000000 cash=10000000
[Admin] notice broadcast delivered=5 text=...
[Admin] room disbanded room=12 members=3
```

还没有落库的审计表 —— 需要新增一张表（改结构后记得重新生成 `internal/store/schema_expected.go`），等以后一起做。
