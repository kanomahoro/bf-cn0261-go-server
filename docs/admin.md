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

- `enabled` 为 `true` 时启动时多监听一个端口；`token` 少于 16 个字符会启动失败
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

浏览器打开这个地址，第一次会让你输入 `token`（存在浏览器本地）。

## 能做什么

| 页面 | 内容 |
| --- | --- |
| 概览 | 在线玩家数、房间数、中继连接数、运行时长、数据库延迟、版本与构建提交 |
| 在线玩家 | 昵称、账号、等级、位置（广场 / 房号+座位）、IP、在线时长；**踢下线 / 封号** |
| 房间 | 房号、标题、模式、地图、人数、阶段、是否锁房；**强制解散** |
| 日志 | 内存里最近 5000 条，按关键字过滤（`[Room]`、`[Relay]`、`[Admin]`…） |
| 查玩家 | 按账号名 / 昵称 / 账号 ID / 角色 ID 搜索，显示等级、金币、点券、封禁状态 |
| 战绩 | 对局列表 + 每局详情（泡封/击爆/营救/经验/金币/等级），可导出 CSV |
| 操作 | 全服公告；改金币 / 点券（最简版） |

## 两个要知道的行为

**封号**：写 `accounts.disabled`、作废该账号尚未使用的登录票据，并立刻断开在线会话。
封号只影响新的登录，已经在对局里的会话会被断开。

**改金币 / 点券**：只执行一条 `UPDATE character_stats`，**不刷新内存、不下发属性包**。
玩家重新登录、在商店买一次东西、或打完一局结算后才会看到新数值。上限金币 1 亿、
点券 2147483647，超了服务端直接拒绝。

## API（需要 `Authorization: Bearer <token>`）

```
GET  /api/overview             GET  /api/players        POST /api/players/kick
POST /api/players/ban          GET  /api/rooms          POST /api/rooms/disband
GET  /api/logs?limit=&q=       GET  /api/search?q=      GET  /api/results?mode=&limit=&offset=
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
