# 配置说明

配置只有一份文件：`config/server.json`（`--config` 可指向任意路径）。JSON **严格校验**：
未知字段、类型不符、取值越界都会在启动时报错退出，不会静默忽略。

## 数据表路径（顶层）

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `gameMapsFile` | `config/game_maps.json` | 地图表 |
| `gameWeaponsFile` | `config/game_weapons.json` | 武器表 |
| `gameDefenseFile` | `config/game_defense.json` | 防具表 |
| `gameCharactersFile` | `config/game_characters.json` | 角色 / 频道 / 模式表 |
| `gameRespawnFile` | `config/game_respawn.json` | 复活延迟表 |
| `progressionFile` | `config/progression.json` | 等级、奖励、评级表 |

## `network` — 游戏端口（26100）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `listenAddress` | `0.0.0.0` | 监听地址；本机测试可改 `127.0.0.1` |
| `listenPort` | `26100` | 游戏/大厅端口 |
| `maxConnections` | `256` | 并发连接上限 |
| `idleTimeoutSeconds` | `0` | 连接空闲超时（秒）；**`0`（默认）表示不按空闲断开**，死连接由 TCP keepalive 回收 |

每个接入连接都开启 **TCP keepalive**（空闲 15s 起探测、每 15s 一次、4 次无响应判死），
所以把空闲超时设成 `0` 也不会留下半死连接：真掉线的对端由内核在约 75 秒内报错断开（15s 起探 + 15s×4 次），
只是静默但活着的客户端不再被计时踢掉（Fantasy 就是 `0` + keepalive 这套）。

## `relay` — 对局中继（26104）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否启动中继；关掉后无法进行对局 |
| `listenAddress` / `listenPort` | `0.0.0.0` / `26104` | 监听地址与端口 |
| `advertisedAddress` | `127.0.0.1` | **会真实下发客户端**：开对局时写进 S601 的 4 字节 IPv4。对外开服必须填公网 IP |
| `maxConnections` | `256` | 并发中继连接上限 |
| `idleTimeoutSeconds` | `0` | 中继连接空闲超时；**`0`（默认）表示不按空闲断开** |

## `database` — MariaDB

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `host` / `port` | `127.0.0.1` / `3306` | 数据库地址 |
| `databaseName` | `bubblefighter_cn0261` | 库名，迁移里写死，改名需同步改 SQL |
| `username` | `bfserver` | 运行账号 |
| `password` | 空 | 留空则读环境变量 `BF_DATABASE_PASSWORD`（配置文件优先） |
| `connectTimeoutSeconds` / `queryTimeoutSeconds` | `5` / `5` | 连接与查询超时 |
| `poolSize` | `4` | 连接池大小 |
| `tlsMode` | `disable` | `require` / `disable`；本地 MariaDB 通常没开 TLS 就填 `disable` |
| `verifyCertificate` | `false` | `tlsMode: require` 时必须为 `true`，否则配置校验失败 |
| `caFile` | 空 | 自签 CA 路径，用系统信任链时留空 |

## `authentication` — 登录器认证端（26102）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否启动认证端；关掉后登录器无法取票 |
| `listenAddress` / `listenPort` | `0.0.0.0` / `26102` | 监听地址与端口 |
| `certificateFile` / `privateKeyFile` | `config/authentication_*.pem` | TLS 证书与私钥，用 `./gen-cert.sh` 生成 |
| `gameHost` | `127.0.0.1` | 预留字段，**服务端不下发**（只做格式校验） |
| `inviteCode` | 空 | 注册邀请码；留空 = 开放注册，填了则必须一致（常量时间比较） |
| `registrationsPerHour` / `registrationsPerDay` | `5` / `20` | 按客户端 IP 的注册配额；`0` 表示用默认值 |

## `logging`

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `level` | `info` | `trace`/`debug`/`info`/`warn`/`error` |
| `directory` | `Logs` | 日志目录，不存在会自动创建 |
| `maxFileSizeBytes` | `5242880` | 单文件 5 MB 触发轮转 |
| `maxFiles` | `3` | 保留 3 份历史，合计约 20 MB 上限 |

日志同时写控制台与文件；目录不可写会导致**启动失败**，不会静默降级。
`debug` 会记录每个进/出包，排查协议问题很有用，但一天能写满 5 MB，日常建议保持 `info`。

级别约定（改日志时照着来）：

| 级别 | 写什么 | 例子 |
| --- | --- | --- |
| `info` | 一局的骨架：开局、结束、结算落库；一局对局下来几行而已 | `[Round] started`、`[Round] ended`、`[Round] settlement clamped` |
| `warn` | 玩家能感知、但服务端自己扛过去的问题 | `game result packet rejected`、`character reload failed`、`loading timed out`、`[Relay] registration timeout` |
| `error` | 数据写不进去、连接 panic、启动失败 | `settlement failed`、`startup failed` |
| `debug` | 单个包的故事：进/出包、复活细节、被规则丢弃的包、重复到的确认包 | `inbound`、`[Relay] inbound`、`[Respawn] completed`、`[Respawn] confirmation ignored` |

原则：**INFO 回答「这一局发生了什么」，DEBUG 回答「这个包发生了什么」。** 复活属于后者
——一局可能复活几十次，对开服的人没有信息量，所以 `[Respawn] completed` 放在 DEBUG，
一行说清（下发的 `spawn`、客户端回传值、是否采纳、状态是否推进）；重复到达的确认包同样
只在 DEBUG。

**一局竞技/道具局在 INFO 下通常只有 3 行**：`[Round] started`、`[Round] ended`、
`[Round] cancelled`；出现夹断时再多一行 `[Round] settlement clamped`。

僵尸局按同一原则留四行骨架（各自一局只出现一次）：

| 行 | 意思 |
| --- | --- |
| `[Round] started` | 开局，带双方人数与时钟 |
| `[Zombie] the king turned` | 本局的僵尸王是谁（含候选人类） |
| `[Zombie] result decided` | 结算判定（人类胜利/僵尸胜利） |
| `[Round] ended` | 回合结束，带比分与原因 |

僵尸局里**可能重复发生**的事件一律在 DEBUG，别往 INFO 里加：`seat turned`（每次转化）、
`conversion announced`（每次延时换队通知）、`the king fell`、`the king left`、`the last human left`、
`the king used its ultra`、`respawn request`（每次死亡）、`box taken` / `item forwarded`（每个箱子/每个道具包）。
道具模式同理：`[Supplies] pickup`、`item used`、`item slot switched`、`box created` 都在 DEBUG，
INFO 只留一局一次的 `[Supplies] map report`。

## `zombie` — 僵尸模式的换队等待

2015 客户端自己不算复活时间：玩家死亡后由客户端播读条动画，服务端等读条走完再广播换队
通知（S504），客户端收到才「起来」。两个等待是**真机量出来的**，不是推算的。

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `humanTurnMs` | `12000` | 人类死亡 → 通知「你变成僵尸」的等待（人类读条较短） |
| `zombiniTurnMs` | `18000` | 小僵尸再死 → 通知复活的等待（小僵尸读条较长） |

允许范围 500–60000 毫秒，超出范围启动即报错（几乎只能是手滑）。**整个 `zombie` 段可以省略**：
缺省时用上面两个默认值，所以 v0.2.0 时代的旧配置不用改也能跑僵尸局。

```json
"zombie": {
    "humanTurnMs": 12000,
    "zombiniTurnMs": 18000
}
```

> 注意 `zombiniTurnMs` 里的 `zombini` 不是笔误 —— 字段从移植那天就这么拼，改它会破坏现有配置。

## `config/` 目录里的数据表

| 文件 | 内容规模 | 管什么 |
| --- | --- | --- |
| `server.json` | 服务端配置 | 上面所有字段 |
| `game_maps.json` | 57 张地图 | 地图池、主题、每图的默认比分上限与时限 |
| `game_weapons.json` | 466 把武器 | 武器 ID 白名单与伤害系数（服务端只校验 ID，不计算伤害） |
| `game_defense.json` | 448 件防具 | 防御修正，开对局时据此生成防御种子表下发 |
| `game_characters.json` | 11 角色 / 12 频道 / 2 模式 | 角色血量修正、频道与模式元信息 |
| `game_respawn.json` | 11 角色 + 10 道具 | 复活等待时长（道具优先于角色） |
| `progression.json` | 10 档等级 | 经验阈值、胜负平奖励、个人分权重、评级与奖牌门槛 |

这些 JSON 都是**运行时读取**，改完重启服务端即生效，不需要重新编译。
