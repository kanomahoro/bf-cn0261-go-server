# 日志级别约定

## 一句话原则

- **Info** = 一局/一次连接/一次管理操作里「值得写进运营日志」的状态变化，量级是「每局个位数」
- **Debug** = 排查才需要的过程细节，可以高频（每包、每次命中、每次刷新）
- **Warn** = 预期外的异常，但服务端已自行兜底、玩家无感
- **Error** = 服务端自己失败（落库失败、编码失败、panic 兜底）

生产跑 `info`；排查时把 `config/server.json` 的 `logging.level` 改成 `debug`，日志按 5 MB × 3 个文件滚动。

## 各模式的事件分级

| 事件 | 级别 | 理由 |
| --- | --- | --- |
| `[Round] started` / `ended` / `cancelled` / `settlement clamped` | Info | 每局一次，运营看的就是这些 |
| `[Bomb] sub-round decided` / `next sub-round` / `settling the decided match` | Info | 每小局一次 |
| `[Bomb] the round starts` / `sub-round results sent` / `revive used` | **Debug** | 与上面重复或每人每局一次，属过程 |
| `[Bomb] no load report … counting down anyway` | Info | 已知常态（子局不重报 C701） |
| `[Tutorial] select` / `stage opened` / `stage started` / `stage ended` / `back to …` | Info | 一次教程每关一次 |
| `[Tutorial] stage list` / `goal counted` / `spawn group echo` / `wave table echoed` | **Debug** | 过程细节，`goal counted` 每击杀一条 |
| `[Tutorial] wave` | **Debug** | 逐波推进，属过程（营救/波次排查才看） |
| `[Supplies] map report` | Info | 每局一次，记录点数与刷新间隔 |
| `[Supplies] 具体转发/拾取` | Debug | 高频 |
| `[Defense] countdown ended without every seed confirmation` | Warn | 有座位没到种子确认，已兜底但值得看一眼 |
| `[Defense] seeds forced without every character report` | Warn | 同上 |
| `[Pirate] the run starts` / `wave` / `wave table` / `every wave cleared` / `stage reported` | Info | 每局/每波一条，多人结算排查靠它 |
| `[Pirate] round start packet` / `spawn frame from the host` / `spawn group from the host` / `drop traffic ignored` | **Debug** | 逐波逐帧的过程细节（含 hex） |
| `[PveBoss] the round starts` / `the map ships no mission groups` / `mission started` / `mission cleared` / `mission timed out` / `mission failed: the citizen died` / `seat turned` / `nobody is left on the human side` / `stage reported` | Info | 每局/每任务/每次转队一条，BOSS 战排查靠它 |
| `[PveBoss] points` / `score unit accepted` / `score unit ignored` / `counted item used` / `mob life point` / `loot box created` / `unclaimed room packet` / `a zombie seat came back` / `the raid starts` / `drop traffic ignored` | **Debug** | 每次加分/道具/血量/未认领包一条，量大（见文末实测） |
| `loading timed out` / `[Bomb] the match settlement was dropped` / `[Bomb] the armed bomb was never reported` | Warn | 真异常，但已有兜底 |
| `[Plaza] dropping a seat whose session is gone` | Debug | 断线清理的常态 |
| `[Channel] move refused: single channel` | Debug | 客户端会重复请求，否则刷屏 |
| `inbound` / `outbound` / `[Relay] inbound` / `[Relay] consumed` | Debug | 协议排查的基石，量大 |
| `peer life` / `peer kick` / `[Life] event dropped` | Debug | 计分排查用（`accepted` / `special` / `direct` 都在里） |

## 输出格式

- 每条 `Info` / `Warn` / `Error` 的消息都以 `[子系统]` 前缀开头：`[Round]`、`[Pirate]`、`[PveBoss]`、`[Bomb]`、
  `[Tutorial]`、`[Zombie]`、`[Captain]`、`[Supplies]`、`[Defense]`、`[Relay]`、`[Session]`、`[Login]`、
  `[Authentication]`、`[Admin]`、`[Shop]`、`[Database]`、`[Application]`、`[Room]`、`[Plaza]`、`[Channel]`
- 结构化字段一律 `key=value`（slog 的 `TextHandler` 输出），键用小写驼峰：`room`、`slot`、`session`、
  `map`、`winner`、`bytes`、`hex`、`error`
- `Debug` 的协议日志保留裸消息（`inbound` / `outbound` / `[Relay] inbound` / `[Relay] consumed`），
  它们本身就是「id + len」的机器可读行，加前缀反而不好 grep
- 一个事件只打一条：`.Info` 里不再重复 Debug 已经写过的细节（例：结算只打 `[Round] ended`，
  逐座位的结果字留在 Debug）

## 别做的事

- **不要在 Info 里打每个包**：`inbound/outbound` 必须留在 Debug
- **不要为「已预期」的情况打 Warn**：警告要能对上「需要人看一眼」的语义
- **同一事件不要同时打两条**（例：`sub-round decided` 已经包含胜负，`results sent` 只在 Debug 留）


## 2026-11-14 复核：Info 里移走的七条

一次两局的真实日志（`Server (16)`，9 分钟）里 Info 一共 22 类，其中三条就占了 74 行：

| 行 | 条数 | 处理 |
| --- | --- | --- |
| `[PveBoss] unclaimed room packet` | **60**（每局封顶 30） | → Debug |
| `[PveBoss] score unit accepted` | 10（每局前 5 条） | → Debug（`[PveBoss] points` 已有同数据） |
| `[PveBoss] loot box created` | 10（每局前 5 条） | → Debug（拾取侧另有 `ammo box taken`） |
| `[PveBoss] score unit ignored` | 0（本局） | → Debug |
| `[PveBoss] the raid starts` | 0（无任务图，不会发） | → Debug（`the round starts` 已是里程碑） |
| `[PveBoss] a zombie seat came back` | 0（本局） | → Debug（僵尸座位每发一次复活请求就一条，客户端可重复触发） |
| `[Pirate] spawn group from the host` | 0（本局是 raid） | → Debug（每个刷怪组一条，含 hex） |

移走后同样这局只剩 **9 条** Info（启动 9 条 + `[Round] started/ended/cancelled` 6 条 + `the round starts` 2 条 +
`the map ships no mission groups` 2 条 + `seat turned` 3 条 + `stage reported` 1 条 + `nobody is left` 1 条 +
`[Supplies] map report` 2 条）。**没有新增任何开关**，只是级别调整。
