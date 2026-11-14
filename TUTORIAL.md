# 教程模式（Tutorial，单人）

**状态：真机通过**（2026-11-11）：落地页 → 选关（25 关全开）→ 编号关卡（mode 15）与 prologue（mode 103）均可完成，
结算与回跳正常。代码：`internal/game/tutorial.go`、`internal/protocol/tutorial.go`。

## 1. 规则与范围

| 项 | 决定 |
| --- | --- |
| 入口 | **登录后的落地页**（教程 / 游戏广场 / 家族）；全程不进广场 |
| 范围 | prologue + 24 个编号关卡 |
| 通关记录 | `922` 恒回「25 关全部已通关」，`best_count`/`best_seconds` 填 0 |
| 奖励 / 数据库 | **不发奖励、零数据库改动**（不建表、不加迁移） |
| 不校验关卡顺序 | 客户端自选 |

## 2. 相位（SessionSetting.Kind）

| Kind | 场景 | 关键点 |
| --- | --- | --- |
| 5 | 选关界面 | `mode=0`，三个图串全空，只放自己一个座位 |
| 2 | 关卡内 | `mode` = 103（prologue）或 15（编号关卡），四串都填 map_key |
| 1 | 落地页 | 退出教程时用（配合 `108 gspGotoMenu`） |

## 3. 包清单

| 用途 | 方向 | 号 |
| --- | --- | --- |
| `gcpReqCreateTutorialSession`（空包） | C2S | 405 |
| `gspGotoRoom` | S2C | 110 |
| `gcpReqSingleStageInfo`（空包） | C2S | 920 |
| `gspResSingleStageInfo`：`u32 count` + count×`(u32,u32,u32)` | S2C | 922 |
| `gcpReqPlayTutorial`（6 字段，见 §4） | C2S | 107 |
| `gspRoomEnterGame` | S2C | 510 |
| `gcpReqEndStage`（1 通关 / 0 失败或退出） | C2S | 724 |
| `gspGameEndGame` | S2C | 705 |
| `gspGameTimeOverEnd`（教程里装的是计分板） | S2C | 732 |
| `gspGotoMenu` | S2C | 108 |

⚠️ 同号不同义：`gspGotoRoom` 是 **S2C 110**，`gspRoomEnterGame` 才是 **S2C 510**；C2S 510 是踢人。

## 4. `107` 字段

线顺序：`13 × u32`（MapMeta 十三字）→ `6 × bool32` → `3 × wstr` → `1 × u32`（difficulty）。

| 字段 | 处理 |
| --- | --- |
| u32 #8 | `TimeAttackTimeLimit`(ms)，**必须 > 0**（为 0 时 letterbox 不消失，也是 S704 的时钟） |
| u32 #11 | prologue 要求 `& 0x10` 非 0；**照收不强制**，只记日志 |
| wstr 1 | `map_directory`，空则回落 map_key |
| wstr 2 | `display`，只记录 |
| wstr 3 | `map_key`，必须是 25 张教程图之一，否则静默丢弃 |
| u32 尾 | `difficulty`，**必须 > 0**（0 时波次表取不到 → 没怪） |

## 5. 进场信封（最容易卡加载）

- **Kind 5**：`mode=0`、图串全空、单座位；mode 103 的目录绝不能搭在选关信封上
- **Kind 2**：
  - prologue → `mode 103`；编号关卡 → `mode 15`
  - GameSetting 四串都填 map_key；`CurrentMap` 空着会在 mode 103 下 AV 崩溃
  - 座位必须推给客户端（`S503 + S505 + S510`）：客户端靠它拿座位号、判断自己是不是 host，
    **不推座位 → 不生成 spawn group → prologue 没怪**
  - 服务端自己发 `510`，不让客户端点准备；倒计时固定 4000 ms

## 6. 关卡数据（MapMeta，已核对）

`isSinglePlayTutorial=1` 共 25 张：`rel_prologue_01` + 24 个 `sin_mission_*`；
`stageNum = 基础号 + 10 ×（难度页 − 1）`。空号：9、10、19、20、29、30。

| 任务组 | 基础号 | stageNum（lv1/lv2/lv3） | 目标 | 限时 |
| --- | --- | --- | --- | --- |
| 泡泡-步枪 | 1 | 1/11/21 | `goalBubbleCount` 15/30/60 | 180/210/240 |
| 泡泡-霰弹 | 2 | 2/12/22 | 同上 | 180/210/240 |
| 泡泡-狙击 | 3 | 3/13/23 | 同上 | 180/210/240 |
| 팡（击爆） | 4 | 4/14/24 | `goalPangCount` 20/40/70 | 180/210/240 |
| 헬프（救援） | 5 | 5/15/25 | `goalHelpCount` 20/40/70 | 180/210/240 |
| 엄폐（掩体） | 6 | 6/16/26 | `goalGateKeeper=1` | 90/60/50 |
| 구르기（翻滚） | 7 | 7/17/27 | `goalPointArrive=1` | 90/45/15 |
| 击爆-狙击 | 8 | 8/18/28 | `goalPangCount` 20/30/40 + `goalPointArrive=1` | 210 |
| 프롤로그 | — | **0** | 波次教程（无 goal 字段） | 99999 |

明细：`workspace/notes/data-bf-tutorial-stages.csv`。

**prologue（`rel_prologue_01`）**：海盗据点救人剧情，纯单人（1/1），首通奖励 `2110130`（我们不发），
怪来自 `NpcMode/rel_prologue_01*.lua` 的刷怪点。它和海盗模式（101）同一套波次链路。

## 7. 局内与结算

- 编号关卡：目标值由客户端读 MapMeta 自判胜负，服务端只做计分回发（§9）
- prologue：需要 NPC 链路（§8）
- 结束都由客户端发 `724`

结算时序：

| 场景 | 动作 |
| --- | --- |
| 编号关卡 | `S705`（3 胜 / 2 负）→ 3 s → `S732`（结果记录：单座位 + 总分 1:0 / 0:1）→ 5 s → `S110 Kind 5` 回选关 |
| prologue | `S705` → 直接 `S110 Kind 1` + `S108` 回落地页，**不发 S732、不等 tick** |
| — | 不发 `gspGameShowResult_`（3D 结算仪式教程用不到） |

## 8. prologue 的 NPC 链路（唯一需要 NPC 的关卡）

- `32857` / `32862` / `32892` 请求**原样回显给发送者**（教程绕过了客户端的本地队列）；
  波次 `903→904`、`904→905`、`908→空 908 + 909`
- `32920 gppLifePoint` **必须回显**：不回则 mob 不扣血、第一下就爆、不起泡
- 教程舞台内的 `14`（health reset）与 `16`（recovery）**必须吞掉**：`14` 会被大厅语义当成 ReqMoveInto
  把玩家拉出去，`16` 会被当成建角请求而断线
- 全部 gate 在教程态，`32xxx` 的通用转发一行未改（普通回合不能回显生成组，否则怪翻倍）

## 9. 目标计数（做，但不落库）

| 上报 | 号 | 计数 |
| --- | --- | --- |
| `gcpGameUpdateScore` | C2S 710（16 字节） | kind 1/22 = 泡泡；kind 4 = 救援 |
| `gcpGameUpdateScoreOptimze` | C2S 2023（6 字节） | kind 2 = 击杀 |
| 回发 | S2C **729** `gspGameBroadcastPlayerScoreData` | 裸 ScoreData，行内 +44 泡泡 / +48 击杀 / +56 救援 |

计数只活在教程态，关卡结束即消失；三个计数都是 ushort，到 65535 不再涨。
`729` 是计分板（verified），`711` 是道具包——别写错。

## 10. 分发顺序（踩过两次的坑）

教程自己的号必须在 `switch packet.ID` **之前**判，否则 `32920` 会被通用的 `handleLifePoint` 先接走：

```
handshake → inbound 日志
  ① tutorialNpcPacketID(id) && handleTutorialNpc(...)   // 32857/32862/32892/32920/903/904/905/908
  ② (id == 710 || id == 2023) && handleTutorialScore(...)
  ③ switch packet.ID（老模式一切照旧）
```

两个闸门只在「会话处于教程舞台」时返回 true。
回归用例：`TestTutorialDispatchClaimsItsOwnNumbers`、`TestTutorialDispatchLeavesTheOrdinaryNumbersAlone`
（走 `Server.Handle`，只测 helper 会漏掉「闸门在 switch 之后」）。

## 11. 明确不做

不建表/不加迁移、不发通关奖励、不发 `gspNotifyAutoTutorial_`（首登自动教程）、
不做 `rel_Tutorial_01` 老教程、不校验关卡顺序、不绑定 `BattleLife`/不进 `finishRound`、
不改 `724` 在合作模式（16/101/102/106）里的语义、不做弹药补给与按关卡限定武器。

## 12. 稳定性与回归

| 约束 | 做法 |
| --- | --- |
| 新号先判态 | `405/920/107/724` 第一行判「是否教程态」，其余会话原样交回 |
| 不改共享写包 | 教程的 `S705` 走独立小函数，不动僵尸局逐人 3/2 那套 |
| 不进回合生命周期 | 教程态不创建 Room，只用会话内存态 |
| 断线清理 | 连接关闭即清教程态（含 3 s / 5 s 定时器，generation 令牌防串局） |
| 空闲踢线 | 无需处理：`idleTimeoutSeconds=0`，存活靠 TCP keepalive |

回归：`go build` / `go vet` / `go test ./...` / `go test -race ./internal/...`，再真机跑一局竞技确认没串味。
