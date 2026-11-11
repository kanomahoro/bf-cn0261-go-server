# Changelog

## 未发布（版本号冻结在 0.8.5，等 master 指令再出号）

- `4e93ade` **狙击模式：武器射爆被泡封的敌人计分**（此前完全不计分）。客户端把「射爆泡泡」上报为
  `C32795` 的 `special=true` 确认，我们一律拒绝（既要求 `pending` 又拒绝 special）。现按模式放行：
  只有狙击 12 接受，且只要求「受害者被泡封 + 对手 + 成功」；接受后置 `kicked`，后续 `GotKicked` 不会重复计分。
  其他模式收到 special 仍照旧拒绝。
- `fa9f70f` 诊断日志（debug 级）：`peer life` 打 kind / victimState / direct，`peer kick` 加 special 与 style。
- `803e41e` **爆破：弹药箱**。爆破地图有补给点，4 个座位每个小局都会上报 `C702`（一局共 12 次），
  但爆破不在 `ammoSupplyMode` 里，上报被丢弃、箱子从未生成。现与其他弹药模式同链路；
  另外 `startSupplies` 每轮重置 `ActiveID`，否则上一小局残留的箱子 id 会让该点下一小局整局不刷。
- `405db4a` **爆破：整场结算不发 S732**。第三局结束后 `endAnnouncement` 阶段广播 S732，而爆破的
  `teamScores` 恒为 0:0，胜利方非房主在 2.84 s 后掉线（真机日志 `artifacts/logs/2026-11-11/bomb-2v2-3rd-round-crash-2218.log`）。
  现 `sendsWinnerAnnouncement(mode)` 让爆破跳过；S729 结果屏 / S733 / S736 链条不变。
- **爆破定稿**：小局结果横幅与 HUD 队伍比分不再尝试（见 `BOMB.md` 的验收结论）。

## 未发布：教程模式（单人，真机已通过）

首次实现教程模式（设计稿 `TUTORIAL.md`），全部为新增代码，老模式一行未改：

- 入口：落地页 `405`（必须空包）→ `S110 Kind 5` 选关（mode 0、三个图串全空、单座位）
- 关卡列表：`920` → `922`，恒回 25 关全通关（stageNum 0/1–8/11–18/21–28，记录填 0）
- 开一关：`107`（13 字 + 6 flag + 3 wstr + difficulty）→ 校验 25 关表 / difficulty>0 / TimeAttack>0
  → `S110 Kind 2`（prologue=103，编号关卡=15）+ `S505` + `S503` + `S522` + `S510`（状态 4）
- 载入：`C701` → `S701` + `S702` + `S703(4000ms)` → 4 秒后 `S704(TimeAttackMs, 0, TimeAttackMs)`
- 结束：`724` → `S705`（3 通关 / 2 失败）；编号关卡 3 秒后 `S732`、再 5 秒回选关；
  prologue 直接回落地页（`S110 Kind 1` + `S108`）
- prologue 的 NPC 链路全部 gate 在教程态：`32857/32862/32892` 回显、`903→904`、`904→905`、
  `908` → 空 `908` + `909` 回显、`905` 当通关处理；现有 32xxx 转发一行未改
- 不建表、不发奖励、不发首登自动教程（S2C 11）、不建 Room、不进 `finishRound/cancelRound/S729`
- 用例：`internal/{game,protocol,world}/tutorial_test.go`（405 非空包、922 25 行、107 校验、
  103/15 信封、载入→开赛、结算 tick、prologue 回落地页、非教程态 724 无副作用、NPC 回显、离开、generation 令牌）
- **2026-11-11 真机验收通过**（prologue + 编号关卡全流程）；随后修掉三处：计数回发包号 711→**729**、
  `S732` 改裸 ScoreData、入场删掉 `S522`、prologue 结束不再多发 `S108`
- **明确不做**：弹药补给与「按关卡限定武器」（Fantasy 同样不做；客户端自己处理，不影响通关）

## v0.8.5（2026-11-11，relay 改动收敛到爆破）

复查「对远程 main 老模式的影响」后，把两条共享的 relay 改动收窄成只在爆破生效：

- **预留存活期回到 2 分钟**（所有模式保持 v0.5.0 的行为）；只有爆破通过 `Extend` 把它推到 10 分钟，
  并且只在小局入场时推
- **同 key 重复注册的「接管」只对标记过 reusable 的 key 生效**（只有爆破标记）；
  其它模式仍然是「一条预留只能一个连接」，重复注册照旧被拒绝
- 接口相应改为 `MarkReusable(key)` + `Extend(key, ttl)`（原来的 `Refresh` 去掉）
- 用例：`TestRelayRegistrationTakesOverARebuiltTransport`（reusable 才接管）、
  `TestRelayExtendKeepsTheReservationUsable`、`TestRelayDuplicateRegistrationStaysRejected`（普通 key 仍拒绝）

## v0.8.4（未采用，已撤回）

试过用 `gspUpdateScoreData`（S2C 986）在小局之间播系列胜场，真机反馈**这套也有问题**（master：好像之前试过），
已整段撤回；`CHANGELOG` 保留本条仅作记录。当前有效版本是 v0.8.3。

## v0.8.3（2026-11-10，小局之间不再发 S732）

按 master 的判断直接收敛：**爆破小局之间只发回合结束包（S705，字 = 1），不发 S732**。
这一版对应稳定形态（横幅在小局之间不显示，但不会崩），把精力留给后面真正的 HUD 比分（入场包内嵌 ScoreData）。

- 删除小局结束的 S732（含 0:1 / 1:0 / 1:1 三种情况）
- 保留：r21 relay 预留 10 分钟 + 每局刷新、r23 普通回合结束包（字 1）、r26 中途退出改取消回合
- 用例改为「小局判出只能出现 S705，不得出现 S729 / S732」

## v0.8.2（2026-11-10，平局小局不发 S732）

v0.8.1（S732 放本小局胜负 `[1,0]/[0,1]`）真机结果：**1:1 那一次仍然崩**，
只是从「两边一起崩」变成「赢的那一方崩」——日志里会话 2（slot 1）在判出后 **2.57 秒** 被强制关闭。

三次对照足够说明问题不在字节内容：

| 版本 | 1:1 时发的 S732 | 结果 |
| --- | --- | --- |
| v0.7.4（完全不发） | — | 稳定，进第三局，只是没横幅 |
| v0.7.9 | `[1,1]` | 两边闪退 |
| v0.8.1 | `[0,1]` | 赢的一方闪退 |

- 现在：**系列打平（wins 相等）时跳过 S732**；其余小局照发（0:1 / 1:0 横幅已验证正常）
- 代价：1:1 那一小局的结束横幅看不到（屏幕本来就显示 1:1，不画也不缺信息）
- 用例：`TestBombTiedSeriesSkipsTheTally`

## v0.8.1（2026-11-10，S732 改成「本小局谁赢」）

v0.8.0 用「相等就跳过」临时规避了 1:1 崩溃。按参考实现把这两个字节的含义确认清楚后改成更根本的写法：

- `RoundTimeOverTotals` = `red_total: u8` + `blue_total: u8`（两队总分，饱和 255）；
  参考实现在爆破里用 `RoundWins` 填这两个字节
- **本小局有赢家，所以这两个字节放「本小局胜负」**：1 队赢 → `[1, 0]`，2 队赢 → `[0, 1]`
  —— 永远不可能相等，客户端崩溃的那个“无赢家”分支从根上不会出现
- 累计胜场（真正的系列比分）不应从这一包出，它属于入场包内嵌的 ScoreData（尚未实现）
- 用例：`TestBombSubRoundsSendTheOrdinaryRoundEnd`（1 队赢 → `[1 0]`）、
  `TestBombTallyStatesTheSubRoundWinner`（2 队赢 → `[0 1]`）

## v0.8.0（2026-11-10，1:1 时不发 S732）

真机 v0.7.9：小局横幅回来了（S732 确实是横幅来源），但 **1:1 时双方又闪退**。
前一次 1:0 的 S732（`[1,0]`）没事，1:1 的 `[1,1]` 崩 —— 两个字节相等就是触发条件。

- **S732 只在两个值不相等时发**：1:1 那一次跳过（屏幕上本来就显示 1:1，不需要它画什么），
  以避免这个崩溃；其余小局（0:1、1:0、0:2、1:2 等）照常发，横幅正常
- 用例：`TestBombEqualSeriesScoreSkipsTheTally`

## v0.7.9（2026-11-10，找回小局横幅：回到 S732）

把历史捋了一遍：**r16/r17（0.6.8/0.6.9）那两轮，master 的反馈是「小局结算是对的」**，
那时发的正是 **逐人 S705 + S732**；从 v0.7.1 起为了别的修复把 S732 删掉，横幅就再也没出现过。
v0.7.8 用「延迟 S729」也没能唤回横幅（客户端不吃这一套）。

所以这一版把组合收敛成：

- **S705 广播普通回合结束包（字 = 1）** —— 解决「一边战斗结束、一边时间结束」（word 2 = 时间到）
- **紧跟一条 S732（两个字节 = 当前系列胜场）** —— 这是横幅的来源，也是 r16/r17 时横幅正常的原因

不再发小局 S729（它会顶掉横幅），也不再需要延迟定时器。`bombRoundPause` 保持 8 s。

## v0.7.8（2026-11-10，小局结果横幅 + 中途退出的容错）

真机 v0.7.7（间隔拉到 15 s）仍然看不到小局结果横幅 → 排除「时间不够」，确认**客户端是在等结果包**。

- **恢复小局记分板，但延迟发送**：小局判出时先发回合结束包（客户端播「战斗结束」），
  **5 秒后**再发 S729 记分板（带当前胜场，`bombScoreboardDelay`）——这一次客户端才会播小局结果横幅；
  **8 秒后**才推下一小局入场（`bombRoundPause` 15 s → 8 s）。决胜局仍由结算那一拍发记分板
- 行内容与结算那套一致（座位/账号/昵称/队伍/等级段/经验段，奖励字段留空）
- **修「中途退出把对面搞崩」**：第二小局里进攻方中途退出后，我们按两队制走「弃权结算」，
  给剩下的人发了一整套结算（S729 + S705 + 511 + 502 + 732），爆破客户端在系列中途收到这个会崩。
  现在爆破模式遇到「一侧清空」改为**取消回合回房间**（和掉线取消同一条路径），不再中途结算

## v0.7.7（2026-11-10，小局间隔 8 s 仍不够，试 15 s）

- v0.7.6 把 `bombRoundPause` 从 5 s 提到 8 s，真机反馈**仍然看不到小局结果横幅** → 再提到 **15 s**，
  继续测量客户端到底需要多久（或确认光等不够、需要发结果包）

## v0.7.6（2026-11-10，小局之间留出横幅时间）

真机 v0.7.5 反馈：两边文案一致了（都是「战斗结束」），但**立刻**就进了下一小局，只有决胜局能看到小局结果横幅。
对照原版视频：零售版是「战斗结束 → 小局结果 → 下一局」三段，我们只有「战斗结束 → 下一局」。

- 只改一个常量：`bombRoundPause` 5 s → **8 s**（小局判出到下一局入场之间的等待），
  给客户端播完「战斗结束」再播小局结果的时间；决胜局本来就在 +5 s 走结算，不受影响
- 这轮是**测量实验**：如果 8 秒能看到横幅，我再把时间往回收，取最短可用值

## v0.7.5（2026-11-10，小局结束改成普通回合结束包）

真机 v0.7.4 反馈：前两小局没有结果横幅，而且**一边显示「战斗结束」、一边显示「时间结束」**。

- **根因**：我们照抄僵尸模式，小局判出后逐个座位发 `gspGameEndGame` 的 outcome 字（赢 3 / 输 2）。
  在这个客户端里 **2 就是「时间到」**（我们自己的 `CreateGameEndReply(true)` 用的也是它），
  所以输方被显示成“时间结束”
- **改法**：小局判出后**广播普通回合结束包** `CreateGameEndReply(false)`（字 = 1，正常结束），
  和所有其它模式的回合结束完全一致；输赢与炸弹横幅由客户端按炸弹状态自己渲染
- 仍然**不在小局之间发 S729**（实测客户端把任何 S729 当最终结果，横幅会被顶掉）；决胜局的横幅来自
  结算时那一次 S729
- 用例改为 `TestBombSubRoundsSendTheOrdinaryRoundEnd`：必须发 S705、字必须是 1（不能是 2/3），
  且不得出现 S729/S732

## v0.7.4（2026-11-10，撤回小局记分板，横幅恢复）

v0.7.2 真机反馈：小局结束的横幅不再显示「已引爆 / 你赢了 / 你输了」，变成通用的「时间到 / 结束」，
并且第二小局的 HUD 仍然没有比分。

- **撤回小局记分板**：v0.7.1 起每个小局判出后都发一块 S729 记分板（照参考实现）。实测这个客户端
  把任何 S729 都当成「比赛结果」处理，于是炸弹横幅被结果面板顶掉。现在**小局之间只发逐人 S705**
  （横幅恢复），积分板只在**整场结算**时由结算流程那一次发送
- 同时删掉 v0.7.1 引入的 `CreateGameResultReplyWithWins` 与 `bombScoreRows`（不再使用）
- HUD 比分（`team_wins`）**仍未实现**：按参考实现它来自**入场包 S510 内嵌的整块 ScoreData**，
  需要单独一轮实现（包体约 6.5KB，风险最高）
- 用例改写：`TestBombSubRoundsSendOnlyTheOutcome`（小局判出只能出现 S705，不得出现 S729/S732）

## v0.7.3（2026-11-10，第三小局 relay 预留过期）

真机 v0.7.2 的第三小局卡在倒计时（客户端提示「频道移动失败」），日志给出了确切原因：

```
23:04:01  [Bomb] series started
23:06:38  [Bomb] next sub-round round=2
23:06:38  [Relay] unknown relay key  slot=0 / slot=1      ← 关键
23:06:42  defense seeds forced without every character report
23:06:42  [Round] cancelled (phase=6 倒计时)
```

- **根因**：relay 预留的存活期只有 **2 分钟**（`Register` 里 `expiresAt = now + 2min`），
  而一个系列是三小局、每小局最长 240 秒，第三小局入口时预留早已过期 →
  客户端用同一个 key 重新注册被判「未知 key」→ 没有 relay → 角色上报到不了 →
  防御种子只能走兜底 → 客户端加载/切场景失败（提示「频道移动失败」）→ 掉线并取消回合
- **修法**：① 预留存活期改为 **10 分钟**（`relayReservationTTL`）
  ② 每个小局入口**刷新**该座位的预留（新增 `Relay.Refresh`，只续期、不动绑定关系）
- 用例：`TestRelayRefreshKeepsTheReservationUsable`

## v0.7.2（2026-11-10，崩溃后的重新登录）

- **修「重启后第一次登录提示服务器断开」**：客户端上一局崩过之后，会先发一个
  **`gcpReportLastCrash`（C2S 19）**再发 FirstConnect（cn-717 `sub_89B820` 构包，原版服务器
  「记录但不回应」）。我们把它当成「首包不是 FirstConnect」直接断开，于是客户端提示服务器断开，
  再登一次（客户端不再带崩溃报告）就好了
- 现在：握手前的 19 号包**记录一行日志并吞掉**，连接保留、继续等 FirstConnect；
  其他未知首包仍然按原样拒绝
- 用例：`TestCrashReportBeforeHandshakeKeepsTheConnection`

## v0.7.1（2026-11-10，小局记分板按参考实现补上）

对比 Fantasy 的小局结束序列后改正：

- 参考实现（`RoundFlow.Result.cs` → `RoundResultSender`）在 **2011 上报结束**的小局里发的是
  **完整记分板**（cn-717 上就是 `gspGameBroadcastPlayerScoreData` = **S729**，body 为整块 ScoreData），
  再发逐座位结局；**两字节的 `gspGameTimeOverEnd`（S732）只在时间到（TimeLimit）时用**，
  而且放的是队伍总分不是胜场
- 我们此前发的是「S705 逐人 + S732 两字节」，**从来没有发过记分板** → 客户端 HUD 没有系列比分，
  1:1 要继续第三局时还挂了（日志：decided 后 2.1 秒双方连接被强制关闭，客户端连 C103 都没发出来）
- 现在：小局判出 → **先广播 S729 记分板**（每座位一行 + 每个队伍第 4 个字写当前胜场
  `ScoreTeam.unknown_228`）→ 再发逐人 S705（胜负横幅）→ **删掉 S732**
- `protocol.CreateGameResultReplyWithWins` 是新增的写包入口，原 `CreateGameResultReply` 行为不变（胜场传 0）
- 用例改写：小局判出必须发出 S729、team1 胜场 = 1、team2 = 0，且**不得**再出现 S732

## v0.7.0（2026-11-10，系列结束的容错与诊断）

真机 v0.6.9 的现象：第 2 小局判出后 **进攻组客户端闪退**，房主转给防御组，防御组被踢回房间等待画面，
系列没有结算。日志显示决定性的一拍：

```
22:27:39 [Bomb] sub-round decided ... wins="[0 2]" match_over=true
22:27:42 [Round] cancelled caller=rooms.go:1743 phase=5   ← 有人离开 → 整系列取消
```

- **修容错**：`roundAfterDeparture` 里，**已决定的爆破系列**（`BombSeriesDecided`）不再被掉线取消；
  保留 `phase=settling`，由结算定时器照常收尾（`finishBombMatch`）。以前这里会把整场比赛作废，
  幸存者被送回房间等待画面，成绩也不落库
- **诊断日志**（临时，定位客户端闪退用）：
  - `[Bomb] sub-round results sent`：小局判出后发了什么（S705 逐人 + S732 胜场）、wins、match_over
  - `[Bomb] settling the decided match` / `[Bomb] the match settlement was dropped`（后者带 generation / phase / started / series_winner，能看出结算为什么没跑）
  - `[Bomb] a seat left with the series decided; the match still settles`
- 用例：`TestBombDecidedSeriesSurvivesADeparture`

## v0.6.9（2026-11-10，按零售客户端纠正：爆破不换边）

- **撤回换边**：这个客户端（国服 2015）的爆破**三小局都是 1 队攻、2 队守**，换边是后续版本的机制
  （master 2026-11-10 实测确认）。`bombSeries.attackingTeam()` 现在恒定返回 1，`defendingTeam()` 恒为 2
- **撤回 v0.6.7 的 S505 重述**：那条是按「客户端没换边」猜的修法，而真实原因是客户端本来就不换边；
  `broadcastRoundEntry` 恢复只发 S501
- 保留 v0.6.8 的 `S732`（小局判出后带系列胜场），HUD 比分仍走这条
- 测试：`TestBombSeriesNeverSwapsSides`（三小局都是 1 攻 2 守）、`TestBombSubRoundPublishesTheSeriesScore`

## v0.6.8（2026-11-10，爆破 HUD 比分）

- **补发系列比分（S732）**：小局结束后除逐人 `S705` 胜/负外，再广播一次 `gspGameTimeOverEnd`
  （S732，两字节）。参考实现里这两个字节在爆破模式放的是 **RoundWins**（`RoundResultSender` 里
  `WriteTimeOverTotals(message.RoundWins ?? message.TeamTotals)`），客户端正是用它显示上方的「1:0」。
  我们此前从不发 S732，所以 HUD 一直是 0:0
- 新增用例：小局判出后 S732 必须携带 `[1 0]`

## v0.6.7（2026-11-10，真机第 1 轮反馈，续 4）

真机 v0.6.6 的现象：第二小局能开局，但**攻守没轮换**、**进攻方装不上弹**、**清掉对面后卡在最后画面**。
三者是一条链，根因在第一条。

- **小局入场重述 GameSetting**：客户端在房间里一直用建房时缓存的那份设置，而 `+172` 的首局标志
  在第 0 局是置位的。参考实现 kr-812 `sub_7FE410` 是「入场前应用 flags」，所以每个小局入场都要发
  `gspRoomChangeGameSetting`（S505）。我们此前只在房主改设置时发 S505，小局入场只发了 S501（房间设置）
  → 客户端永远认为进攻方是 team 1 → 不轮换，且它的装机上报被服务端（第 1 局进攻方是 team 2）拒掉。
  现在 series 入场在 S501 之后、座位包之前补发 S505（带当前小局号、首局标志已清），顺序与参考实现一致
- **装不上弹 / 卡在最后画面**是上述问题的连锁现象：客户端按自己的（错误的）攻守判定装机、
  全灭后发出 `2011 (winner=1, planter=1, defuser=-1)`，而服务端处于 `reset` 且要求 `planter=-1` → 拒绝。
  先按根因修，若这两条仍复现再单独处理（需要新日志确认）
- 新增用例：小局入场的 S505 存在、`+176` 为当前小局号、首局标志已清，且普通入场不发 S505

## v0.6.6（2026-11-10，撤回）

- **撤回 `handleChannelMove` 的改动**：v0.6.4 里把「房间内重选当前频道」从回失败改成回成功，
  当时**没有任何日志证据**，属于自作主张的兜底。r13 的日志证明第二小局真正的卡点是
  relay 重复注册（已由 v0.6.5 修）。频道行为回到原样（房间内一律回失败，拒绝日志保持 Debug）
- 保留：relay 接管（v0.6.5）、爆破图池校验（v0.6.2）、复活每小回合 2 次与 relay 宽限（v0.6.1）、
  模式专属图（v0.6.3）、以及爆破房保留请求主题（v0.6.4 的地图对话框修复）

## v0.6.5（2026-11-10，真机第 1 轮反馈，续 3）

- **修「第二小局没有 relay」**：客户端重建场景后会用**同一个 relay key** 重新注册，
  而 `join` 只允许一个连接绑定预留（`reservation.bound`）→ 直接 `[Relay] duplicate registration rejected`
  → 第二小局双方都没有 relay 传输，角色上报到不了服务端（日志里紧接着就是
  `defense seeds forced without every character report` 与回合取消）。
  现在**接管**旧预留：把被替换的连接标记为「服务端主动关闭」（不算掉线）并关闭，再绑定新连接，
  一个座位始终只挂一条传输。日志 `[Relay] registration took over a rebuilt transport`
- 新增 `internal/net/relay_test.go`：同一个 key 二次注册必须接管而不是被拒

## v0.6.4（2026-11-10，真机第 1 轮反馈，续 2）

- **地图对话框只显示 3 张**：建房时我们把 theme 槽改写成「选中的地图自己的主题」（`rel_Village03_CityHall`
  → Village），客户端的地图对话框就停在该主题页 —— 爆破池里 Village 只有 3 张，点「全部」才刷出 6 张。
  现在**爆破房保留客户端请求的主题**（通常是 `All`），其它模式仍然写解析到的地图主题（有测试分别钉住）
- ~~频道移动~~（**已撤回**：当时没有日志证据，属自作主张。r13 的日志证明真正的卡点是 relay 重复注册）

## v0.6.3（2026-11-10，真机第 1 轮反馈，图池补齐）

- **补齐三个模式的专属图**：客户端 `MapMeta.xml` 里模式 8（僵尸）/ 12（狙击）/ 13（热血个人）的图列表是 **58 张**，
  而竞技/道具/队长/热血狙击是 **57 张**（= 我们 `game_maps.json` 的 57）。多出来的是两张只属于单一模式的图：
  `rel_Stadium03_StardustArena`（僵尸 + 热血个人）与 `rel_pirate02_frontal`（狙击）——
  两者 `isNotemTeamModeMap` / `isItemTeamModeMap` 都是 0，所以之前的 catalog 直接漏掉了它们，
  房主在这三个模式里选到会被拒（`map does not support the normal mode`），随机池也永远抽不到
- 实现：`world.modeExtraMaps`（8/12/13）+ `Maps.HasModeMap`，`Find` 与 `Candidates` 接入；
  `normalizeGameSetting` 的 ordinary-map 校验只对这三组放宽
- **对老模式零影响**：模式 1/3/14/18/20 的候选池、校验结果完全不变（有测试钉住：模式 1 仍是 57 张，
  且拒绝 `rel_Stadium03_StardustArena` / `rel_pirate02_frontal`）
- 测试：模式池数量 1→57 / 8→58 / 12→58 / 13→58 / 20→57；两张图在各自模式可建房、在竞技模式被拒

## v0.6.2（2026-11-10，真机第 1 轮反馈，续）

- **修「爆破开局闪退」**：客户端的建房对话框默认停在启动图（一张普通竞技图），
  `normalizeGameSetting` 的爆破分支原先**不做任何图池校验**（注释写了「以爆破池为准」但没实现），
  于是房间会带着竞技图开局 → 爆破模式加载一张没有爆破回合数据的图 → 客户端硬崩（无提示）。
  现在：①建房/改设置时，池外地图**替换**成爆破池里的同主题候选（不是拒绝，避免默认图把建房整体挡掉）
  ②开局前再校验一次，池外图直接拒绝开局（`StartUnavailable`），并在日志打 `[Bomb] the room's map is not part of the mission; refusing the start`
  ③开局成功打 `[Bomb] the round starts  map=...`，以后闪退一眼就能定位是哪张图
- 新增 `world.Maps.IsBombMap`，2 个用例钉住「池外被替换 / 池内保持不变」

## v0.6.1（2026-11-10，真机第 1 轮反馈）

r8 真机测出的两个问题，改动全部落在 `mode == 19` 内，老模式行为不变。

- **复活改为「每小回合 2 次」**（master 2026-11-10 更正：官方那句「最多复活两次」是**每小局**，不是整场）：
  `bombRevives` 按座位计数，**小局重开清零**；额度内走普通复活路径（`S708` 延迟照 `game_respawn.json`），
  额度用完才广播 `S974` 观察者
- **修「小局推不进下一局」**：客户端重载下一小局时会拆掉 relay 传输，`RelayDisconnected` 立刻释放座位，
  `roundAfterDeparture` 又因 `phase != playing` 直接 `cancelRound`，客户端停在「准备 / 禁止退出」界面。
  现在爆破系列进行中（`BombSeriesActive`）先**保留座位**并起 20 秒宽限（`bombRelayGraceWait`），
  客户端用同一个 relay key 重新注册即可接上；宽限到点仍无 relay 且游戏连接还在，才按原逻辑释放
- **诊断日志**：`[Bomb] series started` / `[Bomb] next sub-round` 带 `map`；复活打 `[Bomb] revive used`；
  relay 宽限打 `[Bomb] relay transport dropped during the series; holding the seat` / `came back` /
  `did not come back`

## v0.6.0（2026-11-10，待真机验收）

爆破模式（`mode 19`）落地：整场三局两胜、每小局攻守互换、小局之间只重载场景、整场只结算一次。
既有模式（1/3/8/12/13/14/18/20）的行为不变：新增行为全部位于 `mode == 19` 分支或状态判空之后。

- **爆破（`mode 19`）**：白名单放行、独立图池（`MapMeta.xml` 的 `isBombmode` 六张，含只属于爆破的
  `rel_zombie_01_bomb` / `rel_Sudden06_Tank`）、系列数据写进 S510 尾部三 word 与 GameSetting `+172/+176`、
  炸弹状态机 0..6（`C2012` → 校验 → 广播 `S973`）、小局结果只认房主上报的 `C2011`、
  局号 0 基且 `attacking = 局号 % 2 + 1`
- **小局之间重载场景**：小局结束逐人发 `S705`（3 胜 / 2 负）→ 停表 → 5 s 横幅 → 局号 +1、攻守互换、
  `generation++`、重新 `bindLife` → 重发入场包（**沿用同一个 relay key**）→ 重新武装加载门等客户端上报 `C701`
- **整场结算一次**：`finishRound` 的判定链新增 `seriesWinner`，优先于比分比较；小局结束不发 S729、不落库、不发奖励
- **无复活**：爆破局的复活请求改为向全房广播 `S974`（场景对象号），本小局内不再复活
- **时间到**：未装弹 → 防守方赢下本小局；已装弹 → 本小局不结束，由炸弹自己的 60 s 决定
- **操作权**：装机限进攻方、拆弹限防守方、只有操作者能完成或中断自己的操作、只有房主能报爆炸；
  坐标只在装机时写入，之后每条上报必须与之一致；操作者离房会广播取消（装机 → 0，拆弹 → 5）
- **比分上限不结束小局**：爆破由目标决定胜负，杀人数不判负
- **兜底**：子局重开后 15 秒没人上报加载 → 服务端自己驱动倒计时并开局；已装弹 70 秒没人上报爆炸 →
  服务端自行判定爆炸；`2012`/`2011` 容许尾部多余字节（严格解析会直接断游戏连接，整场报废）；
  同一小局只广播一次观察者
- **建房校验**：mode 19 以爆破池本身为准，池内那两张 `normalMode=0` 的图不再被固定选图规则拒掉

## v0.5.0（2026-11-09）

队长战（`mode 20`）落地并**真机验收通过**；Relay 注册窗口放宽到 60 秒；管理后台修复翻页与钱包提示。
协议版本不变（717），既有模式（1/3/8/12/13/14/18）的行为不变。

- **队长战（`mode 20`）**：白名单与图池放行（图池就是无道具那批，客户端 `isCaptainModeMap` 与
  `isNotemTeamModeMap` 同一集合）、开局每队点名一名队长（S982）、队长阵亡/离房后转交或清空并在复活后
  重新下发、队长 ULTRA 用 C707（无对象 + kind 104）由 S715 只回本人。开局点名与开局包同一批次立即下发（与参考实现一致），
  之后的转交全部事件驱动。规则与真机记录见 `CAPTAIN.md`
- **队长战日志收敛**：INFO 只留每次任命的 `[Captain] announced`，开局武装、C2018、ULTRA 请求/被拒
  全部降到 DEBUG；建房被拒的日志也回到 DEBUG（保留 `mode`/`subMode` 字段便于排查）
- **队长战点名不再有延迟**：开局点名与开局包同一批次下发（原先 1 秒保险），
  `config/server.json` 与配置文档里的 `captain` 段同步删除
- **Relay 注册窗口 30 秒 → 60 秒**：客户端要等场景加载完才发 `ReqJoinRelaySession`，
  原版窗口是一分钟；注册被砍的后果是该成员整局没有 relay，别人看他站着不动
- **队长战：泡封中的队友不再接任队长。** 真机上出现「两人都是队长、只有一人能放大招」：`gspCreateCaptain`
  给角色打标记且协议没有清除标记的包，客户端自己的转交又跳过被泡封的队友，而服务端把泡封算作可接任 →
  广播了一个客户端不会选的座位。转交资格从 `Standing`（含泡封）收紧为 `Active`，没人合格就空着，
  泡封解除后再给。见 `CAPTAIN.md`
- **管理后台：翻页修好了。** 日志与对局记录两个列表的分页容器 id 写成了 `log-pager` / `res-pager`，
  而 `pager()` 按列表键去找 `logs-pager` / `results-pager`，找不到就静默返回 —— 这两个列表因此**从来没有
  分页控件**（`internal/admin/index_test.go` 现在盯着这个不变量）。同时修「每页条数」输入框在日志页不生效
  （查询用的是上一次服务端回显的值，不是输入框的值）
- **管理后台：钱包输入负数现在给人话。** 两个金额字段原来是 `uint32`，负数会让整个请求体解析失败，
  提示变成 JSON 类型错误；改成有符号读入并单独校验，返回「金币和点券不能是负数」

- **一键安装脚本补上管理后台提示**：装完的汇总里会说明后台默认关闭、怎么打开（`admin.enabled`
  与 `admin.token`）、面板地址是 `http://127.0.0.1:26110/`，以及远程请用 SSH 隧道而不是把
  `listenAddress` 改成 `0.0.0.0`
- README 新增「管理后台（可选）」章节（开启四步 + token 规则 + 拒绝启动的阈值），端口表补上
  `26110`，常见问题新增「管理后台打不开」
- **发布规则变更：Release 不再上传 `bfserver-install.sh`**，只放五个二进制包 + `SHA256SUMS`；
  脚本一律从仓库取（raw 的 `main` 或某个 tag），避免 Release 与仓库两份脚本不一致
- README 的「一键安装」改成 GitHub 惯例的写法：`apt update && apt upgrade` → 装 `wget`/`curl` →
  从 **main 分支**取最新脚本 → 运行；并说明脚本里锁着版本与 sha256，所以从 main 拿到的仍是
  「当前发布版」的安装器，想装旧版本把 `main` 换成对应 tag

## v0.4.0（2026-11-07）

管理后台完善 + 四种新模式（狙击 12、热血个人 13、热血团队 14、热血狙击 18）+ 补给与复活点修正 +
广场幽灵座位修复。协议版本不变（717），既有模式的行为不变。

### Added

- **狙击模式（`mode 12`）可以开局。** 它就是竞技模式的逻辑：两队、房间创建时报的时钟与分数上限、
  不放道具箱，服务端没有任何专属规则；狙击武器的限制在客户端自己做（Fantasy 的 opcode 目录里
  也没有任何狙击专用包）。服务端这一侧只做了三件事：建房白名单放行 12、
  `world.Maps.Candidates` 让 12 走 no-item 图池（客户端 `MapMeta.isSniperModeMap` 覆盖全部竞技图）、
  管理后台的模式名与战绩筛选补上「狙击」
- 地图池不需要新配置：`config/game_maps.json` 里的 57 张图全部落在客户端 59 张狙击图集合内
- **热血团队 / 热血狙击（`mode 14` / `mode 18`）可以开局。** 这两种是去掉「泡封—营救」阶段的竞技模式：
  命中最后一击直接死亡，客户端把击倒上报的 `32787` 里的「直接死亡」标志置真，服务端据此给击杀者
  记 pang（队伍总分同加）——这条规则在 v0.1.0 做竞技模式时就补上了。服务端只多做了两件事：
  建房白名单放行 14/18、`world.Maps.Candidates` 让两者走 no-item 图池
  （客户端 `isNotemTeamModeMap` / `isSurvivalSniperModeMap` 覆盖的正是这批图）。武器限制在客户端
- **热血模式有复活读条**：被击杀后照常走服务端的复活握手（`S708` 延时发点 + 客户端确认），
  不是僵尸模式那种自行复活；死亡上报照常处理
- **热血个人（`mode 13`）的出生点与复活点都由客户端决定**：开局服务端从不下发任何坐标（只发地图名与
  座位），入场位置一直是客户端自己决定的；复活时客户端确认的点也不再按队伍校验，报什么用什么。
  服务端只在客户端没报点时给一个兜底点（整张地图随机、不重复上一个）。热血团队/狙击（14/18）
  不受影响，仍按队伍分组校验
- **热血个人（`mode 13`）可以开局。** 它同样是去掉「泡封—营救」阶段的竞技模式，但没有队伍：
  每个座位自成一方，比分上限按**单人**击杀算。结算按参考实现（Fantasy 的 `SinglePvp`）走**全员平局**
  ——名次由客户端自己按比分排序，服务端只负责每人的统计数据与奖励。为了它参数化了四处写死的两队假设：
  `BattleLife` 的对手判定、开局队伍平衡校验、中途退人不判负、出生点归属（客户端权威）

### Fixed

- **被眩晕的座位吃不下击杀**：手雷或道具效果先把人眩晕，紧跟着那一击来的时候，击倒校验只接受
  `active` 状态，于是这一下被丢弃 —— 人既没被泡封也没死，击杀数与队伍分自然不涨。这就是
  「被手雷打过的角色踢爆不加分」。现在 `hittable()` 把 `active` / `stunned` / `recovering` 三种
  状态都算可被击倒，直接死亡也不再要求先有泡封（真机已复测通过）
- **广场幽灵座位（中继先报的那条路）**：杀进程时游戏连接与中继连接由两个协程分别发现。游戏连接先被
  处理时 `Disconnect` 已经收走座位和账号，中继处理器随后又把角色送回广场 —— 而那条 socket 已经关了，
  再没人会删这个座位。现在中继处理器发现会话已在退出就直接清理，不再送回广场
- **广场幽灵座位（清理窗口那条路）**：`Disconnect` 先清广场、最后才摘账号，中继处理器在这中间通过了
  「账号还在线」的判断，等它跑完 `leaveRoom`（里有同步结算）回来入座时，`Disconnect` 已经结束 ——
  座位又被建出来。现在 `Disconnect` 第一行就置 `closing` 标志，`enterPlaza` 入座前拒绝这种会话。
  广场入座全仓库只有这一个入口，所以管理台解散房间、踢人这些并发路径一并覆盖。
  道具模式杀进程 100% 复现的那个残留就是这条

- **感应地雷的物品 id 用错了**：客户端物品表里 205005 是 **「液态地雷 M01」**（国服没给中文名，等于没上架），
  真正的感应地雷是 **205009「液态地雷SF01」**（slogan「自动感应地雷」）。以前把 M01 当成 SF01 用，
  所以道具能出来但表现不对。粘稠手雷 E01-S 的 205004 本来就是对的。现在物品、武器定义（5009）、
  预置逻辑都改过来了

- **狙击（12）/ 热血狙击（18）没有强制狙击枪**：客户端不会自己限制武器，所以带步枪进房就拿步枪打。现在开局广播座位包之前，
  把每个座位的**主武器槽（18）**换成新手狙击枪（204001 练习狙击枪）——**只改这一局的座位数据**，不写数据库、
  不动背包、不碰外观槽；下一局会从玩家真实背包重新生成。204 段共 18 件物品全部是狙击枪（名称逐一核对过），
  其它段没有狙击枪，所以判定用 `itemID/1000 == 204`

- **热血个人只剩一个人时不结算**：这个模式没有两队可以判负，所以退人时原本只处理「全退光」的情况，
  剩一个玩家时回合会一直挂到时间结束。现在剩最后一人就直接结算——**结算内容和正常打完完全一样**
  （全员平局那一套），不为这种情况另造一个判胜结果

- **热血个人的出生点只用一组（11-18）**：真机实测发现第二组（21-28）**和第一组指向同一批位置**
  （复活在 26 号的人正好站在 18 号上），所以混用两组会让两个人落到同一处。现在只用第一组的 8 个点
  （这 8 个点已实测互不相同，够 8 人房用），标记发放后保留 6 秒，同时复活的人一定拿到不同的点
- **热血个人复活时几个人挤在同一个出生点**：服务端选点时只避开了「自己上一次用过的点」，没有避开别人刚用过的。
  真机日志里出现过三个座位在 6 秒内先后拿到同一个标记（26 号）。现在每个标记发出后会保留 6 秒不再派发，
  同时复活的座位一定落在不同点上；点池用完时退回「最早解冻」的那个

- **狙击（12）、热血狙击（18）、热血个人（13）没有弹药箱**：这三个模式的弹药箱在客户端，服务端只负责
  摆放和广播拾取（和僵尸模式同一套）。现在把这三个模式接进补给线路：房主的点位报告会被接受、
  箱子按间隔刷新、拾取会广播给全场并单独告诉拾取者箱子的种类。热血团队（14）暂不接入

- **「随机地图」其实不随机**：`world.Maps.Candidates` 按配置顺序返回，建房和开局两处都取 `candidates[0]`，
  所以只要模式和主题相同就永远是同一张图；而且开局解析出的图名会被钉回房间设置，同一房间连打也一直用
  同一张。现在改成**每局开局重新抽一张**，随机房打完回房间时把钉住的名字清掉，房间卡恢复显示「随机」
- **「随机热门」会让建房失败**：以前只认 `""` 和 `Random`，客户端发来的 `Random_Hot` / `Random_Hot_Dif1`
  被当成具体地图名去查，找不到就报 `unknown map resource`。现在按 `Random` 前缀识别随机族，并把客户端
  发来的标记原样存进设置第四个字符串——它只影响显示，「随机热门」按等概率抽（原版的权重表没有搬过来）

- **空服务器上报错**：`ListResults` / `ResultPlayers` 用 `var out []T` 声明，没有记录时 Go 把 nil slice 编码成 `null`，
  「战绩」「日志」「已封禁」三个列表一 `map` 就抛异常（新服第一次打开必现）。服务端改成返回 `[]`，
  前端也加了 `listOf()` 兜底
- **「原因」列永远显示「比分」**：`game_results.end_reason` 以前写死 1。现在按 `timeExpired` 写
  `2`（时间到）/ `1`（比分）；表的 `CHECK (end_reason IN (1,2))` 本来就允许两个值
- **房间「阶段」显示「未知」**：新建但还没开局的房间内部 phase 还是 0，`phaseName()` 没有这个分支。
  现在建房时就置为 `phaseOpen`，并且 0 也归为「等待」
- **房间「锁」列看不懂**：这一列原本是 `PasswordRequired`（房主有没有给房间设密码，客户端列表里
  就是那把锁），值和房间是否开局无关。标题改成「密码」，值改成「有密码 / 无密码」并带 tooltip；
  同一张表补上房主（显示昵称，悬停看座位号）
- **战绩详情要滚到页底才看得见**：改成左右两栏，点表格任意一行直接在右侧展开结算数据（窄屏自动
  上下堆叠并滚动到详情）；选中行高亮
- **导出 CSV 的鉴权头写错了**（`Authorization: ***<token>`），导出一律 401；改成 `Bearer <token>`
- **管理后台根路径是 catch-all**：`/favicon.ico` 这类请求也会拿到整页 HTML 加 200，现在除 `/` 外一律 404
- **按钮点了没反应**：任一请求失败时内联 `onclick` 的 Promise 没人接，现在统一挂
  `unhandledrejection` → 右下角 toast 显示错误
- **文档写错了 CSV 的鉴权方式**：`docs/admin.md` 说导出走 `?token=` 查询参数，实际服务端
  **从不接受 URL 上的 token**（那会把 token 写进浏览器历史和日志）
- **对着已经消失的行操作时，界面会留一行幽灵**：踢人 / 封号 / 解散的目标可能在你点之前就没了
  （玩家下线、别的管理员解散了房间）。服务端本来就回 404/400，但前端只在成功时才刷新列表，
  那行会一直挂在屏幕上。现在无论成败都重载列表（失败时 toast 说明原因），并且请求在途时按钮置灰，
  连点不会发两次

### Added

- **六个列表都能翻页**（在线玩家 / 房间 / 战绩 / 日志 / 查玩家 / 已封禁）：以前只看得到最新 N 条，更早的根本看不到。
  服务端 `players` / `rooms` / `logs` / `search` / `bans` / `results` 统一返回 `{total, offset, limit, items}` 并接受
  `limit` + `offset`；页面统一渲染「上一页 / 第 x / y 页 · 共 N 条 / 下一页」
  - 日志翻页时自动关掉「自动刷新」（否则 5 秒后窗口又滑回最新），翻回第一页再勾上即可
  - 列表变短（比如最后一页唯一的账号被解封）时自动退回最后一页，不会停在空白页
  - 每页条数可调：战绩 10~200、日志 10~500

### Changed

- **管理后台默认关闭，token 清空**：仓里的 `config/server.json` 现在带 `"enabled": false` 与
  `"token": ""`。后台本来只绑 `127.0.0.1`，但“开箱即开 + 仓库里公开的示例 token”这个组合
  不该是默认值。开启方式见 `docs/admin.md`（启用时 token 必须自己填，否则服务端直接拒绝启动）。
  旧的示例 token 已进入公开提交历史，**不要再用**
- **「操作」页改金币会写错人**：原界面没说要填角色 ID，填反了（昵称/账号名）或只填一个金额时，
  另一种货币会被写成 0。现在明确标注要填**数字角色 ID**，并强制先点「核实」——核实后显示昵称
  与当前金币/点券，金额框自动预填，未核实时「写入钱包」禁用；从「查玩家」点「改钱包」进来会
  自动核实
- **「查玩家」「在线玩家」两个表都加上了角色 ID 列**（等宽字体、点一下复制），这是改钱包需要的值
- 「查玩家」和日志过滤框**回车即查**
- 布局整理：表格统一包滚动容器（窄屏不撑破页面）、抽出的 `.panel` / `.split` 双栏、页签加一句
  说明、按钮分 primary / danger、数据库异常时把错误文本显示出来

### 日志

- **被丢弃的命中现在说明原因**：`[Life] event dropped` 带上事件类型、来源座位与双方状态，
  「这一下为什么不记分」不用再猜
- **踢击日志同时给出双方状态**：`peer kick` 增加 `victimState` / `actorState`，
  泡封—踢爆链路出问题时能直接看出卡在谁的状态上
- **复活点下发是 debug**：`[Respawn] offered` 记录服务端每次给出的标记，排查「几个人叠在同一个点」时用
- **狙击装配也是 debug**：`[Sniper] loadout forced` 每个座位一行，`info` 级别下不进日志——
  一局 INFO 仍然是 3 行骨架（僵尸局 4 行），只有出现夹断时才会多出 `[Round] settlement clamped`
- 日志约定与关键事件速查补进 README（子系统前缀、级别含义、以及该 grep 哪几句）

## v0.3.2（2026-11-06）

真机验收通过。这一版加了**管理控制台**，并修掉四类实战问题：广场幽灵座位、同一会话重复退房、
复活装备判断错误、点券上限缺失。

### Added

- **管理控制台**（独立端口，默认 `127.0.0.1:26110`，与游戏进程共用但互不拖累）：概览、在线玩家、
  房间、日志、查玩家（含已封禁列表）、战绩、操作、配置八个页签。可踢人、解散房间、封禁账号、
  改金币/点券、发全服公告、在线改白名单字段的配置（校验 → `.bak` → 原子替换，失败不落盘）。
  鉴权用 `Authorization: Bearer <admin.token>`；弱 token 拒绝启动；连续 10 次失败封 60 秒；
  响应头带 CSP、`X-Frame-Options`、`nosniff`；页面内不出现 token。详见 `docs/admin.md`
- **房间密码**：C402 携带的密码文本写入 `room.password`，加入时比对（不匹配回错误码 4），
  空文本即开放房；房间列表带房主昵称
- **新手手雷补齐**：205003（手雷）、205004（黏性雷）、205009（感应雷）三件都预置，新角色背包内
  各 100 个，手雷仍穿在槽位 21
- **重复登录接管（kick_old）**：同账号第二次登录踢掉旧会话，不再让两条连接共用一个账号
- **TCP keepalive**：空闲 15 秒开始探活、间隔 15 秒、4 次未应答判死（约 75 秒发现断线），能穿透
  连接里的 `*tls.Conn` 包装；`network` 与 `relay` 的空闲超时允许为 0（默认 0，不因空闲踢人）

### Fixed

- **广场幽灵座位**（真机报障）：退对局回广场时座位已登记进目录，但入座的首个包发不出去就直接
  返回，`inPlaza` 没置上 —— 座位永久占用（别人列表里一直有这个人），重登被拒
  （`character is already present on another connection`）。现在：座位一登记就置标志；任何后续
  失败回滚座位并广播离开；断线**无条件**清座位；登录时清掉残留座位（不用重启即可自愈）
- **同一会话重复退房**：管理员踢人与玩家自己退房（或断线）并发时，两个 goroutine 各跑一遍退房，
  出现重复的离房广播和两次房主移交；`session.transitionMu` 让退房串行化
- **点券上限缺失**：后台写点券只受列宽限制（`2147483647`），现与金币统一为 1 亿
  （`store.MaxCash`），HTTP 层与游戏层各夹一次
- **管理后台三处安全问题**：空 token 时 `ConstantTimeCompare("","")` 恒为真（现在拒绝启动）；
  绕过 `New()` 构造时限流器缺失；写余额只在 HTTP 层夹紧（现在游戏侧再夹一次）
- **复活装备判断**：只有「已装备且数量非零」的装备才缩短复活等待，空槽位不再参与计算
- **对局中途退房不再取消整局**：只有某队一个座位都不剩才判负；loading/countdown 阶段仍取消；
  中继掉线立即释放座位。离开通知不再依赖离房者的死连接
- **金币上限提到 1 亿**，新角色初始 1000 万（原 100 万）
- 管理控制台端口被占用时只记错误日志，不再拖死游戏进程；控制台 panic 有 recover

### Docs

- 新增 `docs/admin.md`；`docs/database.md` 补手工改余额的 SQL；`docs/configuration.md` 补
  `admin` 段与「空闲超时为 0」的语义；README 补控制台入口与已知限制

### Note

- 发布树不含 `_test.go`：本轮的回归测试（广场座位 4 例、后台 8 例、配置 token、重复登录）
  随提交历史保留，需要时从历史取回

## v0.3.1（2026-11-04）

**无协议与玩法改动**：行为与 v0.3.0 完全一致，只做仓库与日志的整理。

### Changed

- **仓库合并**：僵尸模式自 v0.2.0（提交 `46e6033`）起在 `feature/zombie-mode` 分支开发，
  以 `d7c2b4b` 为定稿快进合入 `main`。原先独立的 `bf-go-server-zombie` 目录不再存在，
  此后一个仓库、一条 main，同时含模式 1（竞技）、3（道具）、8（僵尸）
- **日志级别收敛**：按 README 既有约定（INFO = 一局的骨架，DEBUG = 单个包的故事）把僵尸与道具
  模式里「每发生一次写一行」的输出降到 DEBUG —— `seat turned`、`conversion announced`、
  `the king fell/left`、`ultra`、`respawn request`、`box taken`、`item packet forwarded`、
  `[Supplies] pickup/item used/slot switched/box created`。INFO 现在一局竞技/道具局 3 行、
  僵尸局 4 行
- **日志前缀统一**，便于按 token grep：`[Round] cancelled`、`[Round] settlement clamped`、
  `[Room] creation rejected`、`[Supplies] map report`（每个补给点的明细降为
  `[Supplies] map report detail`，DEBUG）
- 文档与默认版本号对齐到 v0.3.1（`cmd/bfserver/main.go` 的默认 `Version`、README badge 与示例）
- **单元测试移出发布树**：`go build` 本来就不把 `_test.go` 编进二进制（带与不带测试文件的构建产物
  逐字节相同），按发布口径只保留发布代码；回归测试需要时从历史取回
  （`git checkout f80a083 -- internal/`）。README 新增「发布流程」小节，写明版本号、产物、
  `BIN_SHA256` 与 Release 资产四处必须同时更新

## v0.3.0（2026-11-03，僵尸模式）

v0.2.0（道具战）之上移植僵尸模式（mode 8），真机验收通过后的定稿版本（提交 `d7c2b4b`，
标签 `v0.3.0-zombie-r13`）。规则、限制与验收清单见 `ZOMBIE.md`。

### Added

- **僵尸模式（`mode 8`）现在可以开局。** 2015 客户端不自带这个模式的规则（kr-812
  `ZombieModeMgr`，`sub_53AB30`），只对服务端下发的原生换队通知（S504）作反应，所以
  服务端主持整局：开局全员人类（team 1）；开局 5 秒后随机一名人类变僵尸王（team 14，
  客户端把第一个进入该队的座位显示成王）；人类死亡上报（C2S 515，客户端在死亡时发空
  换队请求）转成小僵尸并走正常复活流程，小僵尸再死继续复活且不限次数；僵尸王不复活，
  它倒下或离房即判人类胜；没有人类时判僵尸胜；180 秒到判人类胜；客户端自己的结束上报
  被忽略。
- **地图沿用竞技/道具的普通图池**：原版把僵尸大战开在所有无道具地图上（2018 万圣节
  活动原文 "모든 노템전 맵"，韩服最常见的僵尸图「서든어택 제3보급창고」就是
  `rel_Sudden01_Warehouse`）。`Random` 也从普通图池抽；`rel_zombie*` 属于模式 16 的
  PVE BOSS，与本模式无关。人数维持 8 席（4v4 上隄），与世界服的房间宽度无关。
- 结算支持服务端裁决的胜方：S729 写入真实队伍（含 team 14）与 winner=14，僵尸局改用
  逐人 S705 胜负字（3 胜 / 2 负），因为两队比分表达不了 team 14；回到等待房时把
  team 14 复位成 1。落库仍按两队的 CHECK 约束映射（team 14 → 2），**不需要数据库迁移**。
- 复活点使用 team 14 的编号组（141~148），沿用道具战那套队内分段算法。

- **僵尸王大招**：能量满时僵尸王按 SHIFT 发 C707（对象字 -1、种类 104），服务端只校验
  发送者是本回合的僵尸王，并只给它回 S715 `gspPveItemUsed`（三个 u32：seat / kind /
  无意义字），效果由王的客户端自己广播。包体取 cn-717 `sub_7F4370` 的三个 u32。
- **复活按参考实现走**：转换后的座位靠客户端收到重复的 S504 自己复活，服务端默认
  不发复活包（`zombie.serverRespawn: false`）。若实测我们的客户端像竞技/道具那样等
  S708，把这个开关置 true 即可，服务端会用 team 14 的 141~148 出生点组派复活点。

### Changed（静态分析第二轮）

- **复活改为按需应答**：默认仍不主动发复活包（参考实现的路线），但只要僵尸座位自己发
  C713/C32904 请求复活，服务端就标死并按 team 14 的编号组（141~148）下发 S708，
  "客户端等服务端复活"的版本也能跑；`zombie.serverRespawn: true` 保留为主动方案。
- **模式 8 接受 0~15 的子模式**：该模式客户端没有规则，子模式只做回显，避免建房被
  `unsupported room sub mode` 拒掉。
- 结算行的队伍映射下移到 store（线上仍带 team 14）；S504 放开 team 14/15；
  修复转化座位留在 death 状态、战斗流兜底认错人、超时胜的 S705 胜负字等问题
  （详见提交 `0522ad6`）。

- **C515 的包体不再校验**：僵尸局里玩家死亡就是这张包（参考实现 kr-812 `sub_76C090`），
  如果某个构建在包里带上 seat/team 两个字，原来的"必须空包"校验会把死亡上报直接拒掉；
  现在僵尸局不读包体，只认发送者自己的座位（等待房的换队请求仍然要求空包）。

- **僵尸模式的弹药补给已实现**：房主 C702 报点位，服务端按间隔刷箱；双方都能捡，捡到广播
  S712 让箱子对所有人消失，不发放道具（弹药在客户端），也不做队伍限制（僵尸捡=抢走）。
  客户端自建的箱子在僵尸局同样转发登记。日志 `[Zombie] box taken` / `[Supplies] map report`。

- **复活读条修正**：转换通知（S504）不再在死亡瞬间广播，而是延后到复活等待结束（`game_respawn.json`，默认 7s，道具可减）；服务端内部仍立刻改队伍，保证敌我判定和胜负判定即时。这样人类/小僵尸都会先走完读条再变身、复活。
- **真机第二轮**：弹药箱拾取补发 S715（客户端据此生效）已验真机通过；超时判人类胜、选王候选、僵尸局道具转发、复活请求都加了 info 级诊断。

- **复活交给客户端，读条时长按机型实测配**：僵尸模式不再下发复活点（`serverRespawn` 开关已删除），死亡上报后服务端只改队伍并按 `zombie.humanTurnMs`（默认 12000）/ `zombie.zombiniTurnMs`（默认 18000）延时广播变身通知；数字是从死亡到通知的绝对时间，已含基础等待与死亡动画。

### Known limitations

- 普通地图是否带 team 14 的出生点组未实测（只在 `serverRespawn: true` 时才用到）。
- 僵尸模式的结算**没有比分横幅**（原版只显示"人类胜利/僵尸胜利"），所以团队总分不参与显示；
  待真机确认的是"客户端是否认 team 14 这个胜方"（S729 winner=14）。
- 死亡上报假设：客户端在模式 8 死亡时发 C2S 515；若它只发普通死亡上报，
  `LifeDead` 那条兜底会接住。

## v0.2.0

### Added

- **道具战（`mode 3`）现在可以开局。** 2015 客户端自己保有这个模式的规则
  （kr-812 `sub_53AB30` 为它建的是普通 `ModeMgr`），所以服务端只补上原版由
  服务端负责的那几件事，箱子本身仍由客户端表现：
  - **箱子来自客户端，不由服务端刷**：2015 客户端自己生成箱子并通过
    `32832 gppCreateMapItem` 通报（这张包在客户端表里是**双向**的，所以既可能
    走中继也可能走游戏连接，两条都接），转发时服务端登记编号、种类和**颜色**
    （0 任意 / 1 红 / 2 蓝），拾取仍然是服务端广播 `S712`/`S713`，其他玩家才会
    看到箱子消失。地图点位开局就有箱子、复活后玩家身边刷一个自己颜色的箱子，
    都是客户端这一侧生成的。
  - **地图报告只做记录与校验**：房主的 `C702 gcpGameReportMapData` 仍会被解析
    （点位、种类、队伍、刷新上下限），但**默认不据此刷箱子**。
    这里要分清两件事：箱子的刷新本身是正常的，**由客户端负责**（房主客户端按
    地图资源自己生成、按自己的间隔补）。
    参考实现的代码里确实挂着一条服务端刷箱路径（`SupplyBinding` →
    `RoundSuppliesReported` → `StartSupplies`，2015 版也会挂载，它只被
    `modern_rooms` 之外的公共分支排除在外），但那条路径**必须先收到房主的 C702
    才有位置可刷**；作者反馈它在他的实现里刷不出箱子（最可能是 2015 客户端的
    道具战根本不发 C702），所以实测表现就是“服务端不生成、只记账转发”。
    实测（2026-11-02 真机日志）修正了这个判断：房主的 C702 **确实会到**
    （`points=4 refused=0 intervals=true`，种类和队伍全部通过校验，说明十五个向量的
    顺序猜对了），而 2015 客户端**并不会自己放地图点位的箱子**——一局里桌子上一
    个都没有。所以服务端刷箱现在是**默认开启**的：开局立刻在每个点位放一个箱子，
    之后按各自间隔补。
  - **未知箱子也认**：既然箱子由客户端创建，服务端没登记过编号的拾取请求同样
    成立（种类要在 `ItemSpecItem` 清单内，两个道具槽仍然限制持有量），否则一旦
    客户端换了创建包的时机，整局道具就全捡不起来。
  - **颜色规则**：红/蓝跟队伍**固定绑定**，别队的箱子客户端自己就不让捡
    （kr-812 `sub_6378A0`）。服务端对客户端自建的箱子只按 `32832` 里的颜色字
    记一条 WARN 后放行——颜色字编号尚未在真机确认，映射错了会让玩家连自己的箱子
    都捡不起来；地图点位箱（颜色来自房主报告）仍然硬拒别队的。
  - **拾取与发放**：`C707 gcpEatItem` 通过后广播 `S712 gspItemEaten` +
    `S713 gspAddItemToSlot`。会被拒的只有三种：种类不在 `ItemSpecItem` 清单内、
    箱子是本局**已知点位**上的别队箱子、以及**两个道具槽都占满**（此时箱子留在
    原地给别人）。2015 客户端是**两个**道具槽（参考实现
    `native_item_slot_count: 2`），拾取先填第一个空槽。
  - **死亡不清道具**：手上的道具在死亡后保留（只要没用掉就一直在槽里），所以
    重生流程不碰道具槽状态。
  - **使用与切槽**：`C708 gcpUseItem` 花掉**当前选中槽**并广播 `S714`；
    `C744 gcpSwitchItemSlot` 在另一个槽有道具时切换选中并广播 `S747`（`result=0`），
    没有可切的目标时只回给请求者（`result=1`，客户端保持原选择）。客户端把
    这个操作叫“按 Alt 交换两个道具的顺序”，只有两槽都有道具时才成立。
  - **丢弃/投掷**：`C728 gcpReqCreateItem` 由服务端分配箱子编号并广播 `S711`，
    之后拾取同样走上面的校验。
  - **道具包按原连接回转发**：客户端对同一个 opcode 在游戏连接与中继连接上是
    分开分派的，所以从哪条连接来就必须往哪条连接回。此前一律优先走中继，导致
    使用方的效果包收到了、其他玩家那条连接上却没有——场上表现就是"香蕉／炮台／
    隐身／空袭／变身只有自己看得见，对面毫无反应"。
  - **扔出的道具会清槽**：`C728` 如果是把手上的道具扔出去，服务端必须同时清掉
    该道具占的槽位，否则服务端一直认为两个槽都满着，之后所有拾取都被拒
    （日志里的 `every item slot is taken`），而客户端那边实际只有一个道具。
  - **击倒不再凭空掉问号箱，复活箱保留**：`kind=0` 的随机箱请求有两种来源——
    击倒时和复活后几秒。2015 原版只有后者。真机日志显示复活箱在复活请求之后约
    4.7 秒由**同一个座位**发出，于是现在按"复活窗口"区分：复活后 12 秒内的
    `kind=0` 请求被接受，并且带上该座位的队伍颜色（客户端据此画成红/蓝箱）；
    窗口之外的随机箱请求拒绝。
    真正把手持道具扔出去（`kind` 非 0）始终允许，并且会清掉对应槽位。
  - **4 种"拾取即生效"的道具**（10/11/15/16）不占槽位，单独处理。
  - **`S714` 必须照发**：曾经推断"客户端自己会通知其他人，服务端不必发"，结果是
    一整局道具全都按不出来——使用方客户端要靠这个包才应用效果。已改回始终广播。
  - **内容物种类不再做白名单**：客户端自己决定箱子里是什么，而且它拣起箱子时本地
    就把箱子删了，服务端按种类拒绝只会让"箱子消失、手里什么都没有"。现在任何种类
    都放行，只在遇到本服务端不认识的种类时记一条日志。
  - **发放补两次**（350ms 与 1.2 秒各一次），都只在"该槽仍持有同种道具"时才补：
    真机里客户端偶尔会丢掉第一次发放，一次重发不一定够。
  - **死亡不清空道具槽**（原版行为）：死亡后手上道具保留。
  - **复活时强制清空道具槽**（成品决定）：客户端复活后道具栏是空的，而重发
    `S713` 它并不接受（试过，HUD 依然是空的），于是服务端保留道具只会制造
    "看不见却还能被用掉"的幽灵。现在复活时把该座位的两个槽一起清空，两边一致，
    复活箱可以立刻拾取。
  - **失配自愈改为只覆盖 slot 0**：不再整表清空，避免把玩家真持有的另一个道具抹掉。
  - **补发不发往倒地的座位**：重发的 `S713` 只在座位处于活动状态时发出，避免道具
    在客户端正在走死亡/复活流程时送达（那可能正是"复活后道具栏被清空"的诱因）。
  - **发放补一次**：真机日志里有一次 `S713` 没在客户端生效（那个变形的道具服务端认为
    已持有、客户端根本没显示），于是服务端的两槽记账整局都是"满"的。现在同一次
    发放会在 350ms 后再补一次——**只有在那个槽仍然持有同一种道具时**才补，用掉的
    不会补回来，所以是安全的幂等重发。
  - **拾取不再被服务端的槽位判断挡住**：客户端只会在它认为自己能拿的时候请求拾取，
    而服务端那套两槽记账只要和客户端失配一次（例如某个道具客户端没显示出来），
    之后整局都捡不了。现在拾取一律接受；如果服务端认为两槽都满，就替换 slot 0 并
    记一条 WARN，把状态自愈回来。
  - **未知编号的箱子不再发放**：以前为了让"服务端没见过创建过程的箱子"也能捡，
    对未知编号一律放行；结果重复/过期的拾取请求会多进一个道具，占住槽位、之后
    什么都捡不了（所谓"幽灵槽位"）。现在未知编号只记日志不发放。
  - **槽位：选中的槽空了就取另一个**：客户端保有自己的一套"当前选中槽"，用它
    指定的那个道具用掉之后，下一次使用请求常常指向另一个槽里的道具。服务端原来
    只认"当前选中槽"，于是这次请求被当成"没有道具"丢弃——场上表现就是"槽里明明
    有道具却按不出来"。现在选中槽为空时自动取另一个有道具的槽
    （真机日志里 5 次连续失败就是这条）。
  - **中继上没建模的 32xxx 一律转发**：炮台状态（`32895`）、道具改变角色外观后的
    复位（`32812`）、角色表现数据（`32889`）这些原来都被服务端吃掉了，于是
    "炮台只有自己看得见""金刚变身对面看不到"。现在道具战里凡是中继自己不会转
    发的 32xxx，都由服务端转给其他玩家；中继本来就会转的（移动、伤害等）跳过，
    避免重复。
  - **小地图物件不是箱子信封**：`32832` 除了 40 字节的箱子创建，还会承载客户端
    的小物件（扔出的香蕉是三个字：object / kind / owner）。这类包原样转发、不按
    箱子登记，也不再刷 WARN。
  - **中继侧的道具包**：`32832`/`32824`/`32829`/`32888`/`32894`/`32909`/`32929`
    不在中继白名单里，以前会被静默吞掉。现在只在道具战房间里转发，并且补上了
    成对的效果包：`32830 gppEndMagnet`、`32831 gppEndFreezeCannon`、
    `32833 gppStepOnBanana`、`32882 gppGhost`、`32818 gppStickGrenadeTo`、
    `32899 gppStormDamager`、`32914 gppEndDarkCircle`、`32915
    gppDeleteStormDamagerAOE` —— 缺了"开始"对面看不到效果，缺了"结束"效果永不
    解除（冰冻整局不解就是这条）。转发时从哪条连接来
    就从哪条转发，对端没连中继时退回游戏连接），游戏连接上收到的同样处理。
    `32832` 会顺带解析创建信封：
    能解出但种类不在清单内、或队伍号越界的**不转发**；解不出来的原样转发（布局
    猜错不该让箱子从对端视野里消失）。
- 箱子编号从 `0x62000000` 起分配：避开角色对象（`1000000 + 100000×座位`），并
  把 `0x63000000` 留给参考实现分配给 NPC 击杀 Lucci 掉落的那一段。
- 道具战的箱子种类固定为 cn-717 的 `ItemSpecItem` 十五项（101/102/108/109/115/
  120/123/127/128/130/136/137/138/139/141），`0` 表示随机箱。
- 走中继的那条路也补齐了闭环：客户端自己创建的道具箱会被按自己的编号登记，
  之后对该编号的拾取仍会广播 `S712`/`S713`，不会因为“不是服务端建的箱子”而被丢掉。

### Changed

- `normalizeGameSetting` 现在放行 `mode 1` 与 `mode 3`；换成道具战后地图按
  `itemMode` 标志校验，竞技模式仍按 `normalMode` 校验。其他模式（8/13/16/101
  等）继续拒绝，直到各自的回合逻辑移植完成。
- 版本号升到 `0.2.0`，安装脚本同步到 `v0.2.0` 的发布资产。

### Sanity checks

- 地图报告里的坐标必须落在 `±100000` 内、种类必须小于 `65536`，否则整份报告按
  布局不匹配处理（记 WARN、不生效），避免解析错位时在地图外刷出箱子。
- 宿主给的刷新间隔超过 10 分钟或上下限颠倒时，按默认 15–30 秒处理。
- `C707`/`C728`/创建信封的尾部多余字段只记不报错：布局猜错时不应连带把整局
  拾取功能关掉。
- 拾取/使用/切槽/投掷都只在**对局进行中**生效，并且先确认会话仍持有它所在的
  座位；等待房间、加载阶段和已离开的会话都改不动道具槽状态。地图报告是全流程里
  唯一允许在加载阶段到达的包（房主在地图加载完就发）。

### Verified on a real client (2026-11-03)

All fifteen item kinds of the 2015 item sheet were exercised on real clients and
behave correctly: booster, shield, ghost, needle, freezeCannon, medipack,
camouflage, polymorph, banana, teamAngel, airBomb, turret, bazooka, machinegun and
shieldField. Also verified: the map-point boxes spawn and refresh, the respawn box
appears in the seat's team colour, a knock-down no longer drops a box, a full item
bar does not take a box it cannot hold onto (known caveat below), the two slots
and the Alt switch work, and the item bar is empty after a respawn.

Known caveat: the air strike (kind 136) shows its effect on both clients but deals
no damage. The log shows the client sends no area-effect packet for it (only the
direct-use packet and the object deletions), which this server forwards verbatim,
so the damage path is inside the client; nothing on the server side is missing.

### The itemMode configuration block is gone (2026-11-03)

The four switches only ever described how the item battle was being figured out, and
every one of them is now settled: point boxes are always spawned from the host's map
report, a client-made box with the other team's colour is logged and allowed, the
server always grants the picked item, and only the respawn box is honoured. They are
removed from `config/server.json` and from the `Config` type, and the behaviour is
stated where it is used. A configuration file from an older build has to lose the
`itemMode` block: the loader refuses unknown keys, and the shipped
`config/server.json` matches what this build expects.

### The item grant is sent once (2026-11-03)

A grant used to be repeated at 350ms and 1.2s in case the client dropped it. The
logs show that the client sometimes ignores the grant of the second slot even
though both copies were delivered, so the repeats were removed: the cause is a
client-side acceptance condition, not a lost packet, and extra copies only risk a
phantom item in a slot the client never filled. `supplies.go` documents what to
look for if this is ever chased.

### Known limits

- 这一版的道具战**尚未在真机上验收**。包布局取自 2015 客户端的读取函数
  （报告 `kr-812 sub_5312F0`、创建信封 `kr-812 sub_530570` / `cn-717 sub_5677F0`、
  拾取 `cn-324 sub_4672B0`、丢弃请求、`gspAddItemToSlot` 三字组），其中
  `C702` 的十五个向量按对象布局顺序解出，若真机报告的形状不同，日志里会出现
  `map supply report rejected`，此时箱子由客户端经中继自行同步，模式仍可玩。
- 道具战没有服务端反作弊：命中、伤害与效果判定和竞技模式一样来自客户端上报。


## Unreleased

### Added

- **`tools/bfserver-install.sh`** - a one-shot installer for Debian 12+ amd64
  cloud hosts, aimed at people who have never administered a Linux server. It
  installs MariaDB, creates the database and account, downloads the v0.1.3
  release binary and the matching source tree, applies the fourteen migrations
  in version order, writes `config/server.json` (including
  `relay.advertisedAddress`, which defaults to loopback and would otherwise
  break every remote client's relay connection), generates the 26102
  self-signed certificate, and registers a hardened systemd unit. Verified end
  to end on a clean Debian 12 host: install, restart, re-run as an update, and
  uninstall. The database password is generated, written to
  `config/server.json` (600) and `/root/.bfserver-secrets` (600), and never
  printed or logged - the banner only says where to find it.


## v0.1.3

### Fixed

- **`P32925` (respawn complete over the relay) is now handled.** A client whose
  `C704` was rejected used to stay in the confirmed state for the rest of the
  round, and every later knockdown/stun of that player was refused, silently
  dropping the score for each kill. The completion now arrives on either path.

- **A respawn confirmation could be rejected**, which skipped the `S707` broadcast
  that tells every observer the respawn finished. The client answers `C704` with
  the marker it actually used; when that differed from `S708` the server refused
  it (three times in one session). A marker inside the team block is now adopted
  instead of rejected, so observers stay in sync; anything outside the block is
  still refused.

- **A character could store more experience than the level table allows**, which
  then made `S105` impossible to build at login (`user stat identity or value`).
  Settlement now clamps the total to the last threshold, exactly like the
  reference's `Advance` (`min(experience + gain, thresholds.back())`).

  The result screen (`S729`) is built from the same clamped value, so it can no
  longer show more experience than the database keeps, and both the result row
  and the packet report the gain that was actually applied (zero at the cap). A
  stored value above the table no longer underflows the unsigned subtraction.

### Changed

- **Settlement clamps experience and Lucci through one shared helper**
  (`store.ClampGain`), so the database row, the result screen and the stored
  balance cannot drift. The stored baseline is pinned to the cap with
  `store.NormalizeTotal` before the gain is applied, because
  `game_result_players` enforces `before + gain = after` - a clamped total with
  the old over-cap baseline would fail the insert and abort the whole
  settlement. The sum is computed in 64 bits (a stored total near the uint32
  ceiling would wrap when no cap is configured), and the reported level is
  pinned to the table's 1..200 range.

- **The wallet is capped at 10,000,000 Lucci.** Settlement stores the clamped
  total and both the result row and `S729` report the gain actually applied, so
  the interface never promises coins the database did not keep.


- **Logging follows one rule: INFO tells the story of a match, DEBUG tells the
  story of a packet.** A respawn now emits a single INFO line carrying the
  offered marker, the marker the client confirmed, whether the client's value
  was adopted and whether the state machine moved. The repeated `C704` a client
  sends after it is already active is DEBUG only, the single per-respawn line is
  DEBUG as well (dozens per match is noise for whoever runs the server), and the
  raw-body dump used while chasing the marker mismatch is gone. A match now
  announces itself with
  `[Round] started` (team sizes, duration) and `[Round] ended` (winner, both
  scores, why it ended), so a log file can be read as a sequence of matches
  instead of a list of respawns. Four cases that used to hide in the debug
  stream are now WARN because the player notices them: a rejected `S729` result
  packet, a failed character reload after settlement, a failed `S105` refresh,
  and three events that cancel a running round (`loading timed out`,
  `relay transport lost`, `registration timeout`).

- **Relay registration timeout is now 30 seconds** (was 20). A client only sends
  `ReqJoinRelaySession` after its scene finished loading, and machines running
  several clients can exceed the old budget.

### Fixed

- **A player could stand still on everybody else's screen while still taking
  damage.** Every outbound frame was written synchronously under the connection
  lock. The relay forwards one member's packet to the other members in a single
  loop, so a single peer that briefly stopped reading blocked that loop and every
  stream queued behind it. That peer kept sending its own position (its read side
  was fine) and kept applying its own damage and bubble state, but nobody
  received its position.

  The reference never writes inline: `TcpListener::Send` appends each frame to a
  per-session queue and stops the session when the queue would exceed 8 MiB, so
  one slow reader never stalls the server. Both transports now queue frames and
  a writer goroutine drains them; exceeding 8 MiB or 8192 frames closes that one
  connection with a warning instead of blocking the caller.

- **Every respawn used the first spawn marker**, so teammates in a four player
  room were placed on the exact same spot and consecutive respawns always
  repeated it.

  Each teammate now owns a private slice of the team's markers and rotates
  inside it:

      stride      = team_spawn_count / members_of_that_team
      spawn_index = team*10 + teammate_ordinal*stride + respawn_count % stride + 1

  `team_spawn_count` comes from the map catalog and the per-slot counter resets
  every round. Two teammates with eight markers therefore use 1-4 and 5-8 and
  can never land on each other, whatever their death counts are.

  The reference walks one ring for the whole team (`slot/2 + respawn_count`, one
  step per respawn), so two teammates stay one step apart and after a few deaths
  respawn onto the marker the other one is standing on. Two field reports
  followed: first that `slot/2` also collapses for the 0/1 and 2/3 seatings this
  server allows, then that the ring still collided. The private slice fixes both.

- **Newcomers were not balanced onto the smaller team.** `ChooseTeam` equivalent
  now counts both teams and puts the joining player on the one with fewer
  members, so a four player room is `1,2,1,2` instead of `1,2,2,2` - which the
  teammate spawn markers rely on.


## v0.1.1

### Fixed

- **A player could freeze for tens of seconds in the middle of a round.** The
  relay forwarded every packet a member sent to the rest of the room, including
  the control packets the server is supposed to consume. `C8`/`C9`
  (*RelayPeerRequest*) announce that a peer is reachable directly: relaying them
  made the receiving client believe a direct peer link existed, so it started a
  P2P attempt that cannot succeed in a relay-only deployment and **blocked the
  client for tens of seconds**. The match then resumed by itself.

  The relay now forwards an explicit whitelist of in-match peer state
  (`relayForwardable` in `internal/net/relay.go`) and drops `C1`, `C3`, `C8`,
  `C9`, `C14`, `C18`, `C20006`, `C32840` and `C32904` after the game layer has
  seen them. `C32840` matters on its own: the reference echoes it to its sender
  only and never broadcasts it. The whitelist mirrors
  `GameRoundCoordinator::HandleRelayPacket` in the C++ reference.

### Documentation

- README: new "中继转发策略" section listing the whitelist, the consumed packets
  and why `C8`/`C9` must never be relayed, plus a troubleshooting entry for the
  frozen-peer symptom.

## v0.1.0

- First release: login → plaza → room → match → settlement → back to the room.
