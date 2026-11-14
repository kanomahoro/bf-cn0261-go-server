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
| `[Pirate] round start packet` / `[Pirate] spawn frame from the host` / `[Pirate] drop traffic ignored` | **Debug** | 逐波逐帧的过程细节（含 hex） |
| `[PveBoss] the raid starts` / `mission started` / `mission cleared` / `mission timed out` / `mission failed: the citizen died` / `seat turned` / `stage reported` | Info | 每局/每任务/每次转队一条，BOSS 战排查靠它 |
| `[PveBoss] points` / `counted item used` / `mob life point` / `drop traffic ignored` | **Debug** | 每次加分/道具/血量上报一条，量大 |
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
