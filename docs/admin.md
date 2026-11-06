# 管理后台

一个跑在同进程里的小型 HTTP 控制台：Go 侧只读内存态与数据库，写操作走
`game.Server` 的方法或单条 SQL，**不改数据库结构、不改游戏协议**。

## 开启

在 `config/server.json` 里加（或改）这一段：

```json
"admin": {
    "enabled": true,
    "listenAddress": "127.0.0.1",
    "listenPort": 26110,
    "token": "换成你自己的长随机串"
}
```

- `enabled` 为 `true` 时启动时多监听一个端口
- `token` 有**强度要求**：至少 16 个字符、至少 8 个不同字符、不能是 `changeme` / `1234567890123456` 这类占位值，否则启动失败
- 仓库里 `server.json` 自带的 token 是**公开的示例值**，用于本机开箱即用；如果后台端口能被别人访问，**先换掉它**（启动日志会为此打一条 WARN）
- 连续 10 次 token 错误会使该地址被挡 1 分钟（HTTP 429）
- **默认只绑 `127.0.0.1`**。要从别的机器访问，用 SSH 隧道：
  `ssh -L 26110:127.0.0.1:26110 user@server`，然后把 `listenAddress` 保持不动
- 想直接对内网开放，把 `listenAddress` 改成内网 IP，并确保 `token` 够长

启动成功后，控制台会打印一段提示（英文，便于在日志中一眼看到）：

```
--------------------------------------------------------------
 Admin console : http://127.0.0.1:26110/
 Token         : admin.token in config/server.json
--------------------------------------------------------------
```

这段提示是在**端口绑定成功之后**才打印的，所以看到它就意味着后台真的在监听；
如果端口被占用，只会记一条 `[Admin] console stopped` 错误日志，**游戏服务端照常运行**。

浏览器打开这个地址，第一次会让你输入 `token`（存在浏览器本地）。

## 能做什么

| 页面 | 内容 |
| --- | --- |
| 概览 | 在线玩家数、房间数、中继连接数、运行时长、数据库延迟、版本与构建提交 |
| 在线玩家 | 昵称、**角色 ID**（可点复制）、账号、等级、位置（广场 / 房号+座位）、IP、在线时长；**踢下线 / 封号** |
| 房间 | 房号、标题、模式、地图、人数、房主位、阶段、**是否设密码**；**强制解散** |
| 日志 | 内存里最近 5000 条，按关键字过滤（`[Room]`、`[Relay]`、`[Admin]`…） |
| 查玩家 | 按账号名 / 昵称 / 账号 ID / 角色 ID 搜索，显示**角色 ID**、等级、金币、点券、封禁状态；下方列出**已封禁账号**，可直接解封 |
| 战绩 | 左表右详情：点任意一行，右侧直接展开泡封/击爆/营救/经验/金币/等级，不用滚到页底；可导出 CSV |
| 操作 | 全服公告；改金币 / 点券（填**角色 ID**，先用「核实」看昵称确认） |
| 配置 | 在线编辑一部分配置项，保存前校验；保存后需要**手动重启**才生效 |

## 在线配置

「配置」页只开放**一部分**字段：端口、连接上限、空闲超时、中继对外 IP、僵尸等待、日志级别、注册邀请码与限流。

- **保存前会校验**：服务端把合并后的内容写到同目录的 `server.json.new`，用同一套 `config.Load` 校验，
  通过后才备份 `server.json.bak` 并原子替换；任何一步失败都不会动原文件
- **不在线编辑**的：数据库连接与密码、证书路径、文件路径、管理后台自己的端口与 token
- **保存后不会自动生效**：页面弹窗提示需要手动重启服务端
- 没有「重启」按钮，重启请用服务管理器（systemd / NSSM / 手动）

## 两个要知道的行为

**封号**：写 `accounts.disabled`、作废该账号尚未使用的登录票据，并立刻断开在线会话。
封号只影响新的登录，已经在对局里的会话会被断开。

**改金币 / 点券**：只执行一条 `UPDATE character_stats`，**不刷新内存、不下发属性包**。
玩家重新登录、在商店买一次东西、或打完一局结算后才会看到新数值。**金币与点券各按 1 亿上限**
（`store.MaxLucci` / `store.MaxCash`），超了服务端直接拒绝——数据库列本身还能装更多，但运营界面不放开。

要填的是**角色 ID**（数字），不是昵称也不是账号名。界面强制先「核实」：核实前「写入钱包」是禁用的，
并且两个金额框会用当前值填好，所以不会因为漏填一格把另一种货币写成 0。
从「查玩家」点「改钱包」进来时会自动带上该角色并完成核实。

## API（需要 `Authorization: Bearer <token>`）

```
GET  /api/overview             GET  /api/players        POST /api/players/kick
POST /api/players/ban          GET  /api/rooms          POST /api/rooms/disband
GET  /api/logs?limit=&q=       GET  /api/search?q=      GET  /api/results?mode=&limit=&offset=
GET  /api/bans                 GET  /api/config        POST /api/config
GET  /api/results/detail?id=   GET  /api/results.csv?mode=
POST /api/balance              POST /api/notice         GET  /api/server
```

CSV 下载走 `?token=` 查询参数（浏览器直接打开新窗口，带不了请求头）。

## 审计

每个写操作都会在日志里留一行，前缀 `[Admin]`：

```
[Admin] kick account=42
[Admin] account ban changed account=42 disabled=true ticketsRevoked=1 kicked=true
[Admin] balance written character=7 lucci=10000000 cash=10000000
[Admin] notice broadcast delivered=5 text=...
[Admin] room disbanded room=12 members=3
```

还没有落库的审计表 —— 需要新增迁移，等以后一起做。
