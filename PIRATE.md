# 海盗模式（mode 101，wgtHorde）

**状态：已实现，等真机验收**（2026-11-12）。实现与原方案一致，落点见本文件末尾「实现记录」。

- 客户端事实来源：`artifacts/2026-11-04-clientdata/`（国服 2015 `MapMeta.xml`、`HotRandomMap.ini`、`Default.ini`）
- 参考实现：`workspace/tmp/fantasy`（`packets/horde.yaml`、`packets/lucci-drops.yaml`、`packets/horde-lottery.yaml`、
  `NpcBinding.cs`、`RoundFlow.Npc.cs`、`RoundFlow.Lottery.cs`、`profiles/cn-717/opcodes.yaml`）
- 我们自己已有的同类实现：`internal/game/tutorial.go`（prologue 波次回显）、`internal/game/supplies.go`（弹药箱）

> 2026-11-12 master 的决定：**结算落库但不改库结构**、**不发放任何奖励**、**不做实际掉落**、
> **地图池硬编码**（与爆破同风格）、**一次性做完**、**不破坏老代码**。


## 1. 一句话方案

**海盗是「房间版的教学 prologue」**：客户端（房主）自己跑波次管理器和刷怪，服务端只做三件事——
① 放行房间与地图 ② 把房主的波次/NPC 请求按规则回给全房 ③ 结算与掉落/翻牌的记账。
没有服务端 AI、没有服务端刷怪、没有服务端寻路。

这与我们教程 prologue 的实现完全同构，差别只有两点：**多人（≤4 席）需要广播而不是回显**；**大厅建房进房流程**。

## 2. 客户端事实（已从客户端数据核实）

| 项 | 值 | 来源 |
| --- | --- | --- |
| 模式 id | 101（`wgtHorde`，海盗入侵）；102 是万圣节换皮、106 是第三季，**我们的客户端只有 101** | `GameModeId.cs` + master 确认的 11 个可选模式 |
| 地图数 | **33 张**（11 主题 × 3 难度） | `MapMeta.xml` 的 `isHordeModeMap=1` |
| 地图键 | `npc_传奇_地图名NN`，NN=01/02/03 即难度 1/2/3；**资源目录是 `rel_*`**（如 `npc_pirate03_prison02` → `rel_pirate03_prison`） | 同上 |
| 人数 | 1–4（`MinUser=1`、`MaxUser=4`） | 同上 |
| 时限 | `TimeAttackTimeLimit=300` 秒（→ 发送 300000 ms），`TimeAttackEnable=1` | 同上 |
| 道具箱 | `ItemEnable=0`（**不放道具箱，弹药由客户端自己管**） | 同上 |
| 加载等待 | `WaitForLoading=90000`（Space01 三张是 100，明显是笔误，按 90 s 处理） | 同上 |
| AI 脚本 | 只有两张「天使救援」图有 `*_horde.ini`（`rel_pirate03_prison_horde.ini`、`rel_village08_daohouse_horde.ini`）；其余图的怪由客户端内置波次表生成 | 同上 |
| 奖励表 | `HordeModeAwardTable=1..5`（按图不同）；`HordeModeAbuseKillCount_1/N`（15/5、30/10、40/10 等，反刷阈值） | 同上 |
| 难度池 | `HotRandomMap.ini` 的 `HordeLv01/02/03MapCnt=6`，每档 6 张随机池 | `HotRandomMap.ini` 87–110 行 |
| 房主随机标记 | 客户端会发 `Random_Hot_Dif1/2/3`（我们的 `normalizeGameSetting` 注释里已经预留了这条） | `internal/game/channels.go` |

33 张完整清单（键 / 目录 / 主题 / 难度 / 时限 / 奖励表）：`workspace/notes/data-bf-horde-maps.csv`。

> 4 张 `npc_Zombie_*` 是 102（万圣节）的遗留行，`MapIdx=-1`、奖励表 0、无 AI 脚本 —— 不进 101 的池。

## 3. 协议清单（cn-717 实测号）

### 3.1 房间与大堂

| 方向 | 号 | 名称 | 说明 |
| --- | --- | --- | --- |
| C2S | 402 / 404 / 513 / 505 / 515 | 建房 / 离房 / 改设置 / 准备 / 换队 | 现有流程照用，放行 mode 101 |
| C2S | 2025 | `gcpUserHostStart` | **房主开局**（空包）；回复是 S2C 510 `gspRoomEnterGame` |
| C2S | 503 | `RoomReqEnterGame` | 现有开局入口；101 也要能用（先按现有链路，2025 作兼容加） |
| S2C | 510 | `gspRoomEnterGame` | u32 unknown0、u32 unknown1、wstr 地图键、u32 = 4（状态）；教程已用同一张包 |
| C2S | 701 | 地图加载完成 | 照用；101 的加载门要允许**1 人**开局 |

### 3.2 波次（房主驱动，服务端回/广播）

| C2S（仅房主） | 回什么 | 回给谁 | 备注 |
| --- | --- | --- | --- |
| 903 `gcpReqStartNewWave`（wave: u32） | 904（同 body） | **全房** | 房主的波次管理器在等这张，不回就停住 |
| 904 `gcpReqEndWave`（wave: u32） | 905（同 body） | **全房** | 不回客户端出不了本波 |
| 908 `gcpReqWaveAllSpawnMobs`（波次表 body） | 先空 **908**，再 **909**（原 body） | **全房** | 909 是刷怪表；房主靠它清 latch，其他人靠它装表 |
| 905 `gcpReqEndAllWave`（空） | 906（空） | **只回房主** | 全波清完 → 进结算 |
| 724 `gcpReqEndStage`（completed: u32） | 结算 | 全房 | 1=通关 / 0=失败；超时也算失败 |

教程里这五张已经跑通（`internal/game/tutorial.go` 的 `handleTutorialNpc`），101 的差别只是**广播给全部座位**、
**只认房主**、并且**不是教程态**。

### 3.3 NPC 实体（不透明 body，服务端只转发）

| 号 | 名称 | 处理 |
| --- | --- | --- |
| 32857 | `gppCreateMobsFromSpawnGroup` | 刷怪组（一个 u32）——**是否转发给其他人待真机确认**（见 §6.1） |
| 32862 | `gppCreateSpawnGroupMemberMob` | 组成员怪 |
| 32892 | `gppCreateAIObject` | 单只 AI 怪（含 id/姿态） |
| 32920 | `gppLifePoint` | 怪的血量上报；**中继已经在转发**（`internal/net/relay.go`），不要重复转发 |

### 3.4 掉落与翻牌

| 方向 | 号 | 名称 | 说明 |
| --- | --- | --- | --- |
| C2S | 736 | `gcpReqCreateLucci` | 击杀者客户端请求在尸体位置生成 Lucci（kind 1 普通 / 2 大） |
| C2S | 2021 | `gcpReqCreateHordeAward` | 另一种请求：**不带 kind，由服务端选 3/4/5**（1/5/10 Lucci 奖） |
| S2C | 720 | `gspGameCreateLucci` | object_id、kind、xyz、速度 xyz、unknown36（击杀者）→ 全房创建 |
| C2S | 735 | `gcpEatLucci` | 拾取：character_id + object_id |
| S2C | 721 | `gspLucciEaten` | 全房播放拾取 |
| S2C | 734 | `gspGameAwardDialog` | 空包，通关后打开翻牌对话框 |
| C2S | 519 | `gcpReqSelectAward` | card: u32 + seat: u32 |
| S2C | 743 / 744 | `gspRepAwardSelectSuccess` / `Fail` | 成功回 card+seat(+第二张)，失败空包 |
| S2C | 519 | `gspHordeModeRewardList` | **同号不同向**：10 张牌的展示向量 `list<u32>` |
| S2C | 736 | `gspGameReturnAfterAward_` | 空包，翻完回房（我们已有 736 用于回房） |

## 4. 服务端设计

### 4.1 状态

海盗局挂在 `Room` 上（**不新建会话态**，与教程的 `tutorialState` 区分）：

```go
// internal/game/pirate.go
type pirateRound struct {
    generation  int   // 与房间回合令牌同源，防旧定时器串场
    difficulty  uint32
    wave        uint32 // 最近一次被确认的波次
    dropSerial  uint32 // S720 的 object_id 起算，0x64000000 段
    lucciFloor  int64  // 本局掉落的 Lucci 总数（上限 10000，仿 legacy max_round_lucci）
    awardTable  uint32
    gotAward    map[int]bool // 每个座位是否已经翻过牌
}
```

### 4.2 波次转发矩阵（核心）

```
只接受「房主 + 阶段 playing + 本局 generation」的 903/904/905/908；
其它座位发这些一律丢弃并记 debug 日志。

903 → 广播 904（原文）
904 → 广播 905（原文）
908 → 广播 空908 + 909（原文）
905 → 只回房主 906；置 phase=settling，进结算
724 → 结算（completed 决定胜负）
```

`32857/32862/32892` 先按「**回显给发送者以外的所有座位（不含发送者）**」实现并打 debug 日志，
真机看是重影（客户端自己已经 local 应用）还是不出现（需要连发送者也回）——一次就能定。

### 4.3 结算

- 胜：`C905` 或 `C724(1)`；负：`C724(0)` 或时钟到 300 s
- `S705` 逐人 outcome：1 队/0 队玩家用 3（胜）/ 2（负）；cn-717 的 `npc_team_outcome` **未审**，
  先按普通两队的 3/2 发，真机看结果屏文字（这是 §6.2）
- `S729` 记分板：复用现有 writer（8 席上限、team 1/2、分数≤200 都够用）；
  队伍映射：若客户端把人放在 team 0，写行时按 0→2 映射（Fantasy 的 Coop 做法）
- 落库：默认**照常写 `game_results`**（101 是正式模式，有经验/Lucci）；如果 master 要求先观察，加开关跳过

### 4.4 掉落（服务端权威）

- 只接受**击杀者座位**的 C736/C2021（发送者校验），坐标做 ±100000 的 sanity 检查
- C736：`kind` 1/2 → 金额 1 / 100（legacy `lucci_kind_values`）
- C2021：服务端选 kind = `horde_award_kind`（legacy 默认 3 → 1 Lucci），后续按 MapMeta `HordeModeAwardTable` 分级
- 分配 `object_id` 后**广播 S720**；C735 校验 object_id 存在 → 删表 → 广播 S721 → 加钱包（有上限）
- 不做：反刷阈值（`AbuseKillCount`）强制拦截，先只记日志

### 4.5 翻牌（可后置）

最小可用：通关 → S734 → 接受 C519(card, seat) → S743 回 card+seat → 超时未选则代选 → S519 展示向量 → S736 回房。
**不发真奖励**（与教程一致，零迁移），或只发 Lucci。真奖励池（`lottery_workbook` 那套）明确不做，除非 master 要。

### 4.6 与现有模式的关系

- `supplyMode(room)`（让 32xxx 通用转发生效）要对 101 返回 false：海盗不放道具箱，
  否则会把 32832 之类的箱子包当道具转发
- 101 **不加入** `ammoSupplyMode`
- 101 的 32xxx 由 `pirateNpcPacketID` 单独处理，避免动到道具/僵尸的既有分支

## 5. 代码接点

| 文件 | 改动 |
| --- | --- |
| `internal/world/hordemaps.go`（新） | 33 张图 + 主题/难度/时限/奖励表；`Candidates(101, theme)`、`IsHordeMap`、按 `Random_Hot_DifN` 过滤 |
| `internal/game/channels.go` | `normalizeGameSetting` 放行 101；随机标记 `Random_Hot_Dif1..3` 解析成难度；时限固定 300000 |
| `internal/game/rooms.go` | 开局：2025 → S510；允许 1 人开局；跳过两队平衡校验；`WaitForLoading` 用 90 s |
| `internal/game/pirate.go`（新） | 状态、波次转发、掉落、翻牌、settle |
| `internal/game/server.go` | 分发 2025 / 903 / 904 / 905 / 908 / 724 / 736 / 2021 / 735 / 519，**闸门放在 switch 之前**（教程踩过的坑） |
| `internal/game/life.go` / `rooms.go` 结算 | 101 的 outcome/队伍映射 |
| `internal/protocol/pirate.go`（新） | S720/S721/S734/S743/S744/S519 写包、C736/C2021/C735/C519 读包 |
| `docs/*`、`README.md`、`CHANGELOG.md` | 模式表、状态、协议 |

## 6. 未审区（真机一次性定，不猜着写）

1. ~~**32857/32862/32892 走哪条连接、要不要转发**~~ —— **已定案（2026-11-12）**：
   参考实现的 `AcceptRelayFrames = SelectMatchRelay`（`PlayRoomCodec.cs:159`）说明，**只要服务端选了中继传输
   （我们就是纯 relay 部署，`udp_peer_setup: false`），客户端的 gpp 帧（≥0x8000）就走中继连接**
   （`RoundFeedBinding.MountPassthrough` 注释引 kr-812 `sub_80D4C0`、cn-324 `sub_6F7F10`）。
   同处的 `NpcBinding` 又把 **SpawnGroup（32857）排除在 peer feed 之外**（`operation != SpawnGroup`），
   因为房主客户端自己把组展开成成员/AI 帧（kr-1256 `sub_8F8C40`、cn-717 `sub_744160` 只在 master 展开）。
   所以：**转发 32862 / 32892 / 32866 / 32873，绝不转发 32857**；链路走中继（`Relay.SetPeerFrameAllowance`）。
2. **`npc_team_outcome`**：cn-717 的 horde 结果消费方没有审计（`packets/builds.yaml` 明写 pending）。
   先按普通 3/2 发，真机看结果屏是「胜利/失败」还是错字。
3. **玩家队伍号**：`S729` 行里客户端给 101 座位填的是 team 0 还是 1 —— 决定要不要做 0→2 映射。
4. **开局包**：2025（host start）是否必发，还是我们现有的 503 链路已经够。
5. **加载门**：1 人房是否等 701；`WaitForLoading=90000` 是客户端等待还是服务端超时。
6. **时钟**：300 s 是客户端自己倒计时还是靠 S510 的 TimeAttack 字段（我们教程用 S510 第三字段填 ms）。
7. **失败条件**：全灭？超时？客户端 C724(0) 是唯一的失败信号吗。
8. **复活**：海盗里玩家死亡是走普通 C713/C32904 复活，还是 PVE 专用（legacy `gcpPveUseRevival`）。
9. **翻牌时机**：S734 是在结算前还是结算后；`S519` 展示向量是否必须 10 个。
10. **难度奖励差异**：`HordeModeAwardTable` 1..5 是掉什么、要不要按表给；先给 Lucci 或什么都不给。

## 7. 分阶段实施

| 阶段 | 内容 | 产出 / 验证 |
| --- | --- | --- |
| P1 | 白名单 + 33 张图 + 建房/改设置 + 随机难度标记 | 能建房、房间卡显示正确地图、4 人可进 |
| P2 | 开局（2025/503）+ S510 + 1 人开局 + 90 s 加载 | 真机进图，**不崩**；日志里看到 701 与 903 |
| P3 | 波次五张包（903/904/905/908）+ 广播 | 第一波能开始、清完能进第二波；§6.1 的日志一笔定案 |
| P4 | NPC 实体转发（32857/32862/32892） | 其他玩家能看到怪、能打死 |
| P5 | 结算（905/724/超时）+ S705/S729 + 落库 | 结果屏正确，回房正常 |
| P6 | 掉落（736/2021 → 720；735 → 721） | 怪掉钱、能捡、钱包变化 |
| P7 | 翻牌（734/519/743/519/736） | 通关后能翻牌并回房（奖励先不做或只发 Lucci） |

每阶段真机一轮，日志归档到 `workspace/artifacts/logs/`；P3 之前的任何阶段出问题都必须能**只靠 mode 101 分支回退**，
不能影响 1/3/8/12/13/14/18/19/20。

## 8. 明确不做

- 客户端 AI、生成点、波次内容（全部在房主客户端里）
- 反刷阈值拦截（只记日志）
- 真奖励池 / 翻牌物品发放（先空转，或只发 Lucci）
- 102（万圣节）/ 106（第三季）—— 客户端没有
- 服务端波次调度（哪怕房主掉线也不会由服务端接管波次；房主掉线即取消回合）

---

## 9. 实现记录（2026-11-12）

| 项 | 落点 |
| --- | --- |
| 地图池（33 行） | `internal/world/hordemaps.go`（`HordeMaps` / `IsHordeMap` / `HordeCandidates`），`catalog.go` 的 `Find`/`Candidates` 接入 |
| 建房放行、图替换、难度标记、300 s、无击杀上限、4 席 | `internal/game/channels.go`（`normalizeGameSetting`、`pirateDifficulty`） |
| 座位全在 1 队、忽略换队、容量 4 | `internal/game/rooms.go`（`ReserveSlot`、`handleCreateRoom`、`handleChangeTeam`） |
| 开局（2025 与 503 共用）、单人可开、跳过平衡、90 s 加载 | `internal/game/rooms.go`（`handleHostStart`、`enterGame`） |
| 波次与刷怪、结算触发、奖励流量吞掉 | `internal/game/pirate.go` + `server.go` 的闸门（在 switch **之前**） |
| relay-only 部署下的刷怪帧 | `internal/net/relay.go` 的 `SetPeerFrameAllowance` + `Server.RelayPeerFrameAllowed`（只放行 32862/32892/32866/32873，32857 不进） |
| 结算：胜利方、零奖励、逐人 S705 | `internal/game/rooms.go`（`finishRound`、`roundClockExpired`、`roundAfterDeparture`） |
| 弹药箱 | `internal/game/supplies.go`（`ammoSupplyMode` 加 101） |
| 回归用例 | `internal/game/pirate_test.go`（6 + 4 个，`go test ./...` 与 `-race` 全绿） |

真机第一轮要看的顺序：①建房能不能进（不行看 `room creation rejected` 的 mode/subMode）②进图与 903
③其他座位能不能看到怪（决定 §6.1 的转发方向）④清波与结算 ⑤弹药箱 ⑥结果屏胜负文字（§6.2）。
日志里搜 `[Pirate]`：`the run starts` / `wave` / `wave table` / `spawn relayed` / `stage reported` /
`every wave cleared` / `the clock ran out`。

### 实现记录补：relay-only 部署的连接选择（2026-11-12，master 再确认）

> **服务端只有全 relay，没有 P2P** —— 这是本文件的成立前提，也是所有"帧走哪条连接"结论的依据。



master 指出「原作者说过 PVE 模式在有 P2P 与纯 relay 时部分包走的连接不一样」，核对结果与之一致，
已在实现里落实：

- **小号包（903/904/905/908、724、736…）**：一直走游戏连接，服务端在游戏连接上应答（本文件 §3.2 不变）
- **gpp 帧（≥0x8000，含 32857/32862/32866/32873/32892）**：纯 relay 部署下客户端发在**中继连接**上；
  服务端的处理是「把它当成游戏连接来的包转发给其他座位」，也就是中继自己要放行这些号。
  我们的 `relayForwardable` 原本不含它们，所以在实装前这一轮修正为：
  `Relay.SetPeerFrameAllowance(Server.RelayPeerFrameAllowed)`，只对**海盗房里已开局**的
  32862/32892/32866/32873 放行。
- **复活请求（32904，0x8088）也是 gpp 帧**：纯 relay 下它走中继连接，中继按参考实现「交服务端处理、不转发」，
  所以我们把 `HandleRelayPacket` 的 32904 接到 `handleRespawn`（与游戏连接的 C713 同一条路）
- **32857 不转发**：房主客户端自己展开刷怪组，转发组号会让每个怪在别人屏幕上出现两次。
  如果真机上出现「其他座位看不到怪」，先看日志里有没有 `[Relay] peer frame forwarded` 与 `[Pirate] spawn relayed`；
  两条都没有 = 客户端连中继都没发（那就是客户端的 P2P 判定问题，看 `[Relay] inbound` 有没有这些号）。

### 实现记录补二：第二轮自查（2026-11-12 01:2x）

自查按「任何新分支都要在老模式下逐字节无效」的口径过了一遍，改了三处：

1. **C2025 只在海盗房生效**。原先 `handleHostStart` 对所有模式都开局，等于给老模式多开了一条口子
   （若客户端在竞技房也发 2025，会多回一个 S510）。现在非海盗房直接 debug 返回，老模式保持「未处理」。
2. **32904 的中继处理只在海盗房生效**。中继侧接进 `handleRespawn` 本身是对的，但它对所有模式都生效；
   老模式是「游戏连接上的 C713」验证过的，保持原样，只有 101 走中继这条。
3. **C519 回 S744（选牌失败）** 而不是静默丢弃：翻牌框只由服务端从不下发的 S734 打开，
   客户端若自己打开，回一个失败字让它干净退出（仍然不发放任何奖励）。

另外确认了两点不必改：

- **S2C 906** 在 cn-717 的标签表里没有独立行（kr-812/cn-717 只把 906 记成 C2S `gcpReqGameOver`），
  但 kr-1256 证据写明「case 0x38A 的空回复把 NPC 完成状态置 1」，kr-812 的 dispatcher 也是 case 0x38A，
  所以给房主回空 906 是对的；真机若卡在最后一波，优先看这条
- **S2C 908** 在 cn-717 是「角色列表清扫」而不是复位，真正的 latch 清在 909 —— 我们发的顺序（空 908 → 909 带表）
  与教程 prologue 已验证的顺序一致，不改

### 实现记录补三：与 Fantasy + 教程经验的大检查（2026-11-12 01:4x）

逐条把「Fantasy 怎么做 / 教程（真机验证过）怎么做 / 我们怎么做」摆在一起对，又修出三处：

1. **32857 的正确处理**（查了 `TutorialHandler.cs:675-710` 与 `npc_local_spawn_group_builds`）：
   - 教程（mode 103）**必须回显给发送者**：教程绕过了客户端的本地队列（kr-812 `sub_7FF340`），
     这也是我们 prologue 当初验证出来的
   - **普通房间（101）不能回显**：cn-717 属于 `npc_local_spawn_group_builds`，普通回合发送前已经本地应用，
     再回显就变成每组怪出现两次；同时**不能转发给别人**（客户端只在 master 展开）
   - 结论：101 里 32857 = **什么也不做**（只留 debug 日志）。之前的实现两种都试错过，现在按证据钉住
2. **合作结算的比分不能是 0:0**：Fantasy 的 `cooperative_score_outcome_modes: [101]` 会把胜方总分抬到比对手高，
   注释里写明「id-400 一次满清因为总分全 0 被显示成失败」。我们照做：通关记 (1,0)、失败记 (0,1)，
   S729/落库都跟着这对数走（≤200，CHECK 不变）
3. **中继已转发的帧不能二次转发**：`HandleRelayPacket` 的通用分支原来会把海盗的 peer 帧再转一次
   （中继放行 + 游戏侧再转 = 每人两只怪）。现在通用分支加 `!RelayPeerFrameAllowed(...)` 排除
4. **中途有人离开 → 取消回合**：波次由房主的客户端驱动，客户端没有接管机制，缺人（尤其是房主）时继续挂着
   会白等 300 秒，改为回房间（与掉线取消同一条路）
