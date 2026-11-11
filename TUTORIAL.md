# 教程模式（Tutorial，单人）

**设计稿 · 未实现** · 目标版本：**国服 2015**（CN0261，协议 717，与参考实现 Fantasy 的 `cn-717` = 国服 2015-06-18 同版）
**状态**：等 19 爆破真机验收通过后开工；本文件是实现依据与验收清单。

---

## 0. 版本纪律（先读这条）

- 一切**包号与包体布局以我们自己的客户端为准**，参照 Fantasy 的 `profiles/cn-717/` 与 `packets/*.yaml`
- **禁止**把现代 build（kr-1256 等）或其他 CN build（cn-324 / cn-406 / id-400 …）的独有特性、号段、字段当依据
- 同号不同义的坑踩过一次：我们 **S702 = everybody-loaded**，而 cn-279 表里 702 = map item data。新增包先查我们自己的 `internal/protocol`
- C2S 与 S2C 是**两个独立号段**：C2S `510` = `gcpReqKickFromSession`，S2C `510` = `gspRoomEnterGame`，不要串

---

## 1. 规则与范围（master 拍板 2026-11-10）

| 项 | 决定 |
| --- | --- |
| 入口 | **登录后的落地页**（三个入口：教程 / 游戏广场 / 家族）。**不从广场进**，教程全程不碰广场 |
| 范围 | **prologue + 24 个编号关卡一起做** |
| 通关记录 | **`922` 恒回「25 关全部已通关」**，`best_count` / `best_seconds` 填 0（全开放、不虚构成绩） |
| 通关奖励 | **不发**（不写 `character_items`、不做库存/防重复/过期） |
| 数据库 | **零改动**：不建表、不加迁移、不动 `database/` |
| 首登自动教程 | **不发** `gspNotifyAutoTutorial_`（S2C 11 空包）→ 落地页永远不弹「要不要玩教程」 |
| 老教程图 | `rel_Tutorial_01`（`isTutorial=1`）那条老路径**不做** |
| 关卡顺序 | 服务端**不校验**；客户端想打哪关就打哪关 |
| 教程里装备 | 玩家**当前角色 + 当前穿戴**（Fantasy 做法），不额外配枪 |
| 其他入口互斥 | **不需要**：客户端在教程选单里切不到广场/家族，退出选单就是回落地页 |
| 实现节奏 | **一次性做完**，一次交付测试包 |

---

## 2. 相位（SessionSetting.Kind）

我们现有语义：**1 = 落地页/主菜单，2 = 房间（播放），3 = 广场**。

```
落地页 (Kind 1)
   │  客户端发 C2S 405（空包）
   ▼
教程选关 (Kind 5, S110)  ── 客户端发 C2S 920 → 回 S2C 922（25 关全通关）
   │  客户端发 C2S 107
   ▼
教程关卡 (Kind 2, S110) + S2C 510
   │  客户端发 C2S 724（结束字）
   ▼
结算：S2C 705 → 3 s → S2C 732 → 5 s → 回教程选关 (Kind 5)
   │  退出选单
   ▼
落地页 (Kind 1 信封 + S2C 108 gspGotoMenu)
```

---

## 3. 包清单（勘误后）

| 用途 | 方向 | 号 | 我们现有实现 |
| --- | --- | --- | --- |
| `gcpReqCreateTutorialSession` | C2S | **405** | 无（新增，**必须空包**） |
| `gspGotoRoom` | S2C | **110** | `CreateRoomEntryReply` / `CreateMainMenuReply` ✅ 可复用 |
| `gcpReqSingleStageInfo` | C2S | **920** | 无（新增，空包） |
| `gspResSingleStageInfo` | S2C | **922** | 无（新增：`u32 count` + count × `(u32,u32,u32)`） |
| `gcpReqPlayTutorial` | C2S | **107** | 无（新增，6 字段） |
| `gspRoomEnterGame` | S2C | **510** | `CreateGameEntryReply` ✅ 可复用 |
| `gcpReqEndStage` | C2S | **724** | 无（新增，1 个字：1 通关 / 0 失败或退出） |
| `gspGameEndGame` | S2C | **705** | `CreateGameEndReply` / `CreateGameOutcomeReply` ✅ 可复用 |
| `gspGameTimeOverEnd` | S2C | **732** | `CreateGameWinnerReply`（2 字节：先胜队 / 后胜队）✅ 可复用 |
| `gspGotoMenu` | S2C | **108** | 无（新增，退出教程回落地页时发） |

**号段勘误记录**（2026-11-10 自查）：早期文档把 `gspGotoRoom` 写成 510 是错的 —— `gspGotoRoom` 是 **S2C 110**，
`gspRoomEnterGame` 才是 **S2C 510**。教程链里 `405 / 920 / 922 / 107` 这四个号一直是对的。

---

## 4. `107 gcpReqPlayTutorial` 字段（**已按 cn-717 线格式修正**，2026-11-11）

线顺序（Fantasy `packets/tutorial.yaml` 的 `TutorialPlayRequest`，cn-717 六条 bool）：

```
13 × u32（MapMetaData 十三字）→ 6 × bool32（flag 字）→ 3 × wstr → 1 × u32（difficulty）
```

| 位置 | 内容 | 服务端处理 |
| --- | --- | --- |
| u32 #8（偏移 40） | `TimeAttackTimeLimit` **ms** | **必须 > 0**；为 0 时 letterbox 不消失，且它也是 S704 的时钟 |
| u32 #11（偏移 52） | `unknown_52` | prologue 要求 `& 0x10` 非 0；**照收不强制**，只打日志 |
| wstr 1 | `map_directory`（Directory） | 空则回落到 map_key；写进 GameSetting 第一个图串 |
| wstr 2 | `display` | 不解释，只记录 |
| wstr 3 | `map_key`（MapMeta 行名） | 必须是 25 张教程图之一，否则静默丢弃（不断线） |
| u32 尾 | `difficulty` | **必须 > 0**（lv_01/02/03 = 1/2/3）；为 0 时 mode 103 波次表取不到 → 没怪 |

> 旧稿把 `unknown_52` 写成独立 u8、把 TimeAttack 写成尾部 u32，是错的：十七个定长字之后才是三个串。

**解析纪律**：`724`、`107` 等共享/新增号一律**先判「是否处于教程态」再解析**，避免把普通房间的连接解析坏（爆破的 2011/2012 已有先例）。

---

## 5. 进场信封要点（最容易卡加载的地方）

### Kind 5（选关）

- `mode = 0`，三个图串**全部为空**
- 只放**自己一个座位**，房主槽位 = 自己
- 理由：mode 103 的 Directory / CurrentMap / map key 绝不能搭在选关信封上（Fantasy `WriteTutorialSelect` 的做法）

### Kind 2（关卡）

- `mode`：prologue → **103**（`wgtHordeTutor`）；编号关卡 → **15**（`wgtSinglePlay`）
- GameSetting：`Directory` = map_key、`CurrentMap` = map_key、map key = map_key、`difficulty` = 请求里的值
  - `CurrentMap` 空着会在 mode 103 下 AV 崩溃
  - `difficulty = 0` → 波次 Lua 拿不到表
- 座位必须推给客户端（**我们现有 S503 + S505 + S522 + S510 这一串**）：
  客户端要从座位包拿自己的座位号；且「第一个座位 == 房主」这个测试决定了它是否把自己当 host，
  **不推座位 → 不生成 spawn group → prologue 没怪**
- 服务端**自己发 S510**（`gspRoomEnterGame`），不让客户端点准备
- 倒计时 **4000 ms**（教程固定），座位号 **0**

---

## 6. 关卡数据（我们自己的 MapMeta，已核对）

来源：`workspace/artifacts/2026-11-04-clientdata/clientdata-serverShared/MapMeta.xml`
提取结果（本地速查）：`workspace/notes/data-bf-tutorial-stages.csv`

`isSinglePlayTutorial = 1` 的图**正好 25 张**：`rel_prologue_01` + 24 个 `sin_mission_*`。
`stageNum = 基础号 + 10 × (难度页 - 1)`。

| 任务组 | 基础号 | stageNum（lv1 / lv2 / lv3） | 目标 | 限时 |
| --- | --- | --- | --- | --- |
| 버블-라이플 | 1 | 1 / 11 / 21 | `goalBubbleCount` 15/30/60 | 180/210/240 |
| 버블-샷건 | 2 | 2 / 12 / 22 | `goalBubbleCount` 15/30/60 | 180/210/240 |
| 버블-스나이퍼 | 3 | 3 / 13 / 23 | `goalBubbleCount` 15/30/60 | 180/210/240 |
| 팡 | 4 | 4 / 14 / 24 | `goalPangCount` 20/40/70 | 180/210/240 |
| 헬프 | 5 | 5 / 15 / 25 | `goalHelpCount` 20/40/70 | 180/210/240 |
| 엄폐（掩体） | 6 | 6 / 16 / 26 | `goalGateKeeper = 1` | 90/60/50 |
| 구르기（翻滚） | 7 | 7 / 17 / 27 | `goalPointArrive = 1` | 90/45/15 |
| 击爆_狙击 | 8 | 8 / 18 / 28 | `goalPangCount` 20/30/40 + `goalPointArrive = 1` | 210 |
| 프롤로그 | — | **0** | 波次教程（无 goal 字段） | 99999（≈不限） |

### 6.1 我们的 prologue 是 `rel_prologue_01`（MapMeta 实测）

| 字段 | 值 |
| --- | --- |
| Theme / Directory | `Forest` / `rel_prologue_01` |
| MapName / MissionName | 프롤로그（序章） |
| **剧情** | 标题「해적에게 잡혀간 친구를 구하자!」＝**去救被海盗抓走的朋友**；描述「去海盗据点救出被关的朋友」；副标题「(必须把海盗全部清光才行。)」；Description「해적의 은신처」＝海盗的藏身处 |
| 人数 | MinUser **1** / MaxUser **1**（纯单人） |
| 时长 | TimeAttack 99999（≈不限），WaitForLoading 90000，LoadingCompleteLimit 45000 |
| 首通奖励 | `IsItemAward = 2110130` —— **我们不发** |
| 结算 tick | `EndGameTimeOverTick 3000` / `EndGameNotifyWinOrLoseTickTeam 6000` / `EndGameNotifyResultTick 11000` |
| 怪从哪来 | `NpcMode/rel_prologue_01*.lua` 的刷怪点（`#spawn_npc01..03` 一类） |

**注意**：prologue **就是海盗模式（101）的剧情序章** —— 同一套「刷怪 / 波次 / 清光敌人」链路，
所以它跑通等于给 101 打了一次最小规模的底（但 101 的多人转发与 rel_/npc_ 双 key 仍需在海盗房间里验）。

- 空号：9、10、19、20、29、30
- `AIScriptFileName` 只有 `supang_sniper_lv_01/02` 有值 → 编号关卡基本纯客户端判定，**不需要 NPC 波次**
- `rel_Tutorial_01`（`isTutorial=1`，Stadium）= 另一条老教程图，**不在**这 25 张里，不用

---

## 7. 局内与结算

### 编号关卡（mode 15）

- 目标值由客户端从 MapMeta 行读取并**自行判定胜负**，服务端不介入胜负
- **但目标计数我们做**（master 2026-11-11 决定，不落库）：见 §16
- 结束由客户端发 **`724`**（1 通关 / 0 失败或中途退出）

### prologue（mode 103）

- 需要 NPC 链路（见第 8 节）
- 结束同样由客户端发 `724`

### 结算时序（照 Fantasy，`tutorial_score_ms` / `tutorial_result_ms`）

| 步 | 动作 |
| --- | --- |
| 1 | 立刻发 **S705**（Won = 字 3 / Lost = 字 2，`CreateGameOutcomeReply`）—— mode 15 靠它显示 GAME OVER cue |
| 2 | 等 **3000 ms** → 发 **S732**，body 用**结果记录（计分板）**：单座位 + 队伍总分 通关 1:0 / 失败 0:1（见下方注） |
| 3 | 等 **5000 ms** → 重发 **S110 Kind 5** 回选关界面（客户端会再发 920） |
| 3' | **prologue（含 905 清完所有波次）**：S705 之后**直接**发 **S110 Kind 1** 信封 + **S108 gspGotoMenu** 回落地页，**不发 S732、不等 tick**（Fantasy `FinishWaveTutorialAsync` 就是这个顺序） |
| — | **不发** `gspGameShowResult_`（3D 结算仪式，教程用不到） |

> 2026-11-11 修正一：旧稿把 prologue 也算进「3 秒 + 5 秒」那条链，实际只有**编号关卡**走结果 tick；prologue 是波次关，结束即回落地页。
>
> 2026-11-11 修正二（master 指出）：**S732 在教程里读的是计分板，不是两个队伍的字节**。普通回合的 `gspGameTimeOverEnd` 在 legacy 家族是 `TeamTotals`（两字节，cn-717 继承 kr-812 基线），但教程的结果舞台要用整块结果记录。实现上直接复用结果记录构造器（与 S729 同一份 body），单座位、`winner` 1/2、队伍总分 1:0 或 0:1。

---

## 8. prologue 的 NPC 波次（唯一需要 NPC 链路的关卡）

- 链路：客户端生成组请求 → 服务端**原样回显**（`gppCreateMobsFromSpawnGroup` 32857 / `32862` / `32892`）；
  波次推进 `904`；mob 创建 / 动作 / 删除转发
- **`gppLifePoint`（C2S 32920）必须回显**：Fantasy 的注释写明「不回显 → mob 不扣血、第一下就爆，不会起泡」。
  2026-11-11 复核补上（原先被丢进 `handleLifePoint` 的 room==nil 分支）
- 教程舞台里的 `14`（health reset）/`16`（recovery）**不能被大厅语义接走**：`14` 会被当成 ReqMoveInto
  把玩家拉出关卡，`16` 会被当成建角请求而断线。现在教程舞台内直接吞掉（只打 debug 日志）
- CN2015 的注解是「tutorial echoes the host C2S body」→ **单人去程照抄回显即可**，不建 NPC 权威状态
- **全部 gate 在教程态**：现有 32xxx 通用转发（模式 8 / 模式 3 在用）**一行不改**
  （Fantasy 曾经改成全局回显，普通回合生成组被重复创建，后来专门修过一次）

---

## 9. 明确不做的事

- ❌ 不建表、不加迁移、不碰 `database/`
- ❌ 不发通关奖励、不写 inventory
- ❌ 不发 `gspNotifyAutoTutorial_`（首登自动教程）
- ❌ 不做 `rel_Tutorial_01` 老教程路径
- ❌ 不做关卡顺序/解锁校验
- ❌ 不绑定 `BattleLife`、不进 `finishRound` / `cancelRound` / `S729`、不做复活与观察者
- ❌ 不改 `724` 在合作模式（16/101/102/106）里的现有语义

---

## 10. 稳定性约束（不影响老模式）

| # | 约束 | 具体做法 |
| --- | --- | --- |
| 1 | 新号先判态 | `724` / `107` / `405` / `920` 的 handler 第一行判「是否教程态」，否则原样交回现有逻辑或静默丢弃 |
| 2 | 不改共享写包 | `S705` 给教程走**独立的小函数**（单座位），不动僵尸局那套逐人 3 胜 / 2 负 |
| 3 | 不改现有转发 | NPC 回显只在教程态开；`32xxx` 通用转发保持现状 |
| 4 | 不进回合生命周期 | 教程态**不创建 Room 对象**，只用内存态 + 包 |
| 5 | 断线清理 | 连接关闭即清教程态（含未完成的 3 s / 5 s 定时器，用 generation/令牌防串局） |
| 6 | 空闲踢线 | 已确认无需处理：`config/server.json` 两处 `idleTimeoutSeconds = 0`，存活靠 TCP keepalive（15 s / 15 s / 4） |

**回归面**：`go build` + `go test ./...` + `go vet` + `go test -race`，再真机跑一局竞技确认没串味。

---

## 11. 实现清单（预估生产 700–900 行，测试 250–350 行）

| 文件 | 内容 | 行数 |
| --- | --- | --- |
| `internal/protocol/tutorial.go`（新） | 读 405 / 920（空包）、读 107（6 字段）、读 724（1 字）、写 922（25 行）、S110 Kind 5 变体、S108 | 150–200 |
| `internal/game/tutorial.go`（新） | 教程态、四个 handler、进场组装（Kind 5 / Kind 2 + 四字段）、结算 3 s + 5 s、回跳、清理 | 350–450 |
| `internal/game/server.go` | 4 个 case + 判态门 | 30–40 |
| `internal/game/tutorial_npc.go`（新） | prologue 生成组回显 / 波次应答 / mob 转发（gate 在教程态） | 150–250 |
| 测试 | 见下 | 250–350 |
| `TUTORIAL.md` | 本文件 | — |

### 单测清单

1. `405` 非空包 → 拒绝且不断线
2. `920` → `922` 正好 25 行，stageNum 与 MapMeta 一致（0、1–8、11–18、21–28）
3. `107` 六字段：缺 `difficulty` / `TimeAttack` → 拒绝；map_key 不在 25 张内 → 拒绝
4. 进场信封：prologue → mode 103；编号关卡 → mode 15；Kind 5 的 mode=0 且三串为空
5. 结算时序：`724`=1 → S705 Won + S732 (1,0) + 5 s 后 Kind 5；`724`=0 → Lost + (0,1)
6. `724` 在非教程态被送进来 → 不影响现有逻辑（不解析、不断线）
7. 断线：教程态清理，定时器不跨局触发（generation 令牌）

---

## 12. 真机验证清单

**第 1 轮（编号关卡）**
1. 落地页点「教程」→ 是否进选关界面（日志：`[Tutorial] select`）
2. 选关界面是否 25 关全亮（因为我们回全通关）
3. 选一关 → 是否加载完成、letterbox 是否消失、有没有武器（当前穿戴）
4. 打满目标 → GAME OVER → 3 s 后成绩画面 → 5 s 后回选关
5. 中途退出（`724`=0）→ 是否同样回选关、不卡

**第 2 轮（prologue）**
6. 进 `rel_prologue_01` → 怪是否刷出来（日志：`[Tutorial] spawn group echo`）
7. 波次是否推进（`904`）、能否打死、能否结束
8. 结束 → 是否回落地页（Kind 1 + 108）

**第 3 轮（如有）**
9. 时序/文案微调；退出选单回落地页是否正常

---

## 13. 风险与兜底

| 风险 | 表现 | 兜底 |
| --- | --- | --- |
| 进场字段填错（中） | 卡加载 | 先只填最少字段，逐个加；对照 `BOMB.md` 的入场序列已验证过一次 |
| 座位包没推 / 房主判定（中） | prologue 没怪 | 日志打「座位包已发 / ownSlot」；先验编号关卡再验 prologue |
| 结算时序（低） | 成绩画面不出来 / 卡在 GAME OVER | `tutorial_score_ms` / `result_ms` 做成可配，真机调 |
| 选关被锁（已规避） | 只能打第一关 | 已选方案 B（全通关） |
| 客户端切场景（已规避） | 教程中切广场 | 客户端做不到，无需处理 |
| 入口首包（已定论） | 落地页按钮发 405 还是 107 | Fantasy 两条路径分明：405 = 按钮→选关界面，107 = 选完关卡/首登自动教程（后者要服务端先发 S2C 11，我们不发）→ **按 405 实现**，真机抓包降级为可选 |

---

## 14. 证据索引（Fantasy，本地 `workspace/tmp/fantasy`）

- `src/Plugins/BubbleFighter.Fantasy.Dialect.Rooms/TutorialHandler.cs`（1166 行：选择 / 进场 / 结算）
- `.../TutorialWire.cs`（377：922 行结构与 107 读取）、`.../TutorialEntryBinding.cs`（85：首登入口）
- `.../PlayRoomCodec.TutorialScores.cs`（110：目标计数，`cn-717: tutorial_score_builds: [cn-717]`）
- `packets/tutorial.yaml`、`packets/tutorial-npc.yaml`
- `profiles/bundles/legacy.yaml`：`tutorial_score_ms: 3000`、`tutorial_result_ms: 5000`
- cn-717 函数地址：`gcpReqPlayTutorial` SETOP `sub_BF35F0(107)`；`405`/`920`/`922`/`107` 均在 `profiles/cn-717/opcodes.yaml`
- cn-717 包号（`packets/tutorial-npc.yaml` / `opcodes.yaml`）：`32857` 生成组（双向）、`32862` 组员 mob（双向）、`32892` AI 对象（双向）、`903→904` 开波、`904→905` 收波、`908`（C2S 波次表）→ 空 `908` + `909` 回显、`905`（C2S 全部波次清完，空包）

---

## 15. 实现记录（2026-11-11，代码已完成，等真机）

| 文件 | 内容 |
| --- | --- |
| `internal/world/tutorial.go`（新） | 25 关表（stageNum / 限时 / prologue 标记）+ `FindTutorialStage`（大小写不敏感） |
| `internal/protocol/tutorial.go`（新） | `ReadTutorialPlayRequest`（13 字 + 6 flag + 3 wstr + difficulty）、`ReadTutorialEndStage`、`CreateTutorialStageInfoReply`（922）、`CreateGotoMenuReply`（108） |
| `internal/game/tutorial.go`（新） | 教程态（挂在 Session 上，**不建 Room**）+ 405/920/107/724 handler + 进场组包 + 701 载入门 + 结算 tick + NPC 回显 + 离开/断线清理 |
| `internal/game/server.go` | Session 新增 `tutorial*` 字段；分发新增 405/920/107/724；NPC 号先判教程态；105（落地页）与 Disconnect 清态 |
| `internal/game/rooms.go` | `handleLeaveRoom` 先问教程；`handleMapLoaded` 先问教程（C701） |

### 与旧稿 / Fantasy 的取值差异（真机 bisect 用）

| 项 | 我们的取值 | Fantasy cn-717 | 出问题时先改这个 |
| --- | --- | --- | --- |
| S110 `State` | 0（选关与关卡都是） | 0 | 改成 2/3 |
| 座位 `team` 字节 | **0**（已与 Fantasy 一致） | 0 | 写入器白名单已加 0（房间局仍只有 1/2、僵尸 14 仍走 S504，出包不变） |
| S510 状态字 | **4**（直接进 playing） | 4 | 改回 3（loading） |
| S705 结果字 | **3 通关 / 2 失败** | 3 / 2 | 已与 `CreateGameOutcomeReply` 对齐 |
| S732 | **结果记录（计分板）**：单座位 + 总分 1:0 / 0:1 | 整块 ScoreData | 已按 master 的「计分板是教程特有」修正 |
| prologue 结束 | S705 → 直接 Kind 1 + S108 | 同 | — |

### 与 Fantasy 仍存的差异（2026-11-11 复核后，全部是「不落库」带来的）

| 项 | 我们 | Fantasy | 原因 |
| --- | --- | --- | --- |
| 目标计数 | 解析并回发 **S729**（裸 ScoreData，行 team=0、+44/+48/+56 三个计数） | 同 | **已一致**；差别只在下面两条持久化 |
| 计数落库 | 不写（只在教程态内存里） | 写 `tutorial_stage_clear` 一类记录 | master：做计数但不动数据库 |
| 通关记录 / `922` | 恒回 25 关全开、best=0 | 返回真实通关行与最好成绩 | zero-migration |
| 首通奖励（MapMeta `IsItemAward`） | 不发 | 按配置发 | 同上 |
| 老教程 `rel_Tutorial_01` + 首登自动教程（S2C 11） | 不支持，`107` 空 map_key 直接拒 | 支持 `old_map_key` 回落 | 明确不做 |
| 教程会话上限 | 无全局上限（每会话一份状态） | `_maxTutorialSessions` | 单人服务器，无资源压力 |
| `S522` 签名 | 教程入场**不发** | 不发 | 已按 Fantasy 删掉 |
| prologue 结束 | S705 → 只发 Kind 1 | 同 | 已按 Fantasy 改；`S108` 只在客户端自己按 `404` 退出教程时补 |
| 载入超时 | 90s（= 回合的 loadingTimeout） | 无超时，一直等 | 防呆，客户端掉线不卡状态 |
| `S732` / `S729` 包体 | 都是**裸 ScoreData**（无奖励尾巴）；732 带 winner 与 1:0/0:1 | 同 | 已按 Fantasy 改 |

### 15.1 分发顺序（2026-11-11 复核修正）

教程自己的号必须在 `switch packet.ID` **之前**判，因为 `32920`（mob 血量）在 switch 里已经有
`case 32920 → handleLifePoint` 会先 return，教程的回显永远走不到（这个 bug 抓过两次，第二次是
helper 级用例掩盖了它）。现在的顺序：

```
handshake → inbound 日志
  ① tutorialNpcPacketID(id) && handleTutorialNpc(...)   // 32857/32862/32892/32920/903/904/905/908
  ② (id==710 || id==2023) && handleTutorialScore(...)   // 目标计数
  ③ switch packet.ID（老模式一切照旧）
```

两个闸门都只在「会话处于教程舞台」时返回 true，其余会话原样落到 ③。用例
`TestTutorialDispatchClaimsItsOwnNumbers` / `TestTutorialDispatchLeavesTheOrdinaryNumbersAlone`
钉住这一点（走的是 `Server.Handle`，不是 helper）。

### 16. 目标计数（2026-11-11 master 决定：做，但不落库）

| 上报 | 号 | 形状 | 计数 |
| --- | --- | --- | --- |
| `gcpGameUpdateScore` | C2S **710** | `u8 kind` + `u32` + `u32` + `u8` + `u8 slot` + `u8` + `u16` + `u16`（16 字节） | kind **1/22** = 泡泡（+knock_downs）、**4** = 救援（+help） |
| `gcpGameUpdateScoreOptimze` | C2S **2023** | `u8 kind` + `u32` + `u8 seat`（6 字节） | kind **2** = 击杀（+kills） |
| 计数回发 | S2C **729** `gspGameBroadcastPlayerScoreData` | 裸 ScoreData（17 队表 + 16 行，**无奖励尾巴**） | 行内 +44 泡泡 / +48 击杀 / **+56 救援**（cn-717 的 help 槽是 `unknown_56`，由 kr-812 基线继承）；opcodes.yaml 明确 729 = 该包（`sub_87EFE0 case 729`，`sub_76C4D0` 正好读 +44/+48/+56），711 是道具包 |

- 计数只活在教程态里（`tutorialState.bubbles/pangs/helps`），关卡结束即消失，**不写数据库**
- 三个计数都是原生 ushort：到 65535 就不再涨（不取模回 0，与 Fantasy 的 `ushort.MaxValue` 守卫一致）
- `tutorialPlay` 在会话已入房时直接拒绝（否则教程舞台会和房间抢 C701/计数包）
- 非教程会话的 710/2023 不拦，照旧走原分发（今天本来就没处理）
- 与 Fantasy 的差别只有「不持久化」

### 回归
`go build` / `go vet` / `go test ./...` / `go test -race ./internal/...` 全绿；老模式分发路径仅在 405/920/107/724 与 NPC 号上新增前判，未改动任何现有 handler 分支。

---

## 17. 真机验收结论（2026-11-11）

**通过**：落地页「教程」→ 选关（25 关全开）→ 编号关卡（mode 15）与 prologue（mode 103）均可完成，
结算与回跳正常，prologue 的刷怪/波次/清场全链路正常。

验证后修正的四处（都已进代码）：

| 项 | 修正 |
| --- | --- |
| 计数回发包号 | `711` → **729**（`profiles/cn-717/opcodes.yaml` 的 verified 条目：729 = `gspGameBroadcastPlayerScoreData`，711 是道具包） |
| `S732` 包体 | 改为**裸 ScoreData**（原先带奖励尾巴，比 Fantasy 多字节） |
| 入场 `S522` | 删除（Fantasy 教程入场不发） |
| prologue 结束的 `S108` | 删除（只发 Kind 1 信封；只有客户端自己按 `C404` 退出时才补 108） |

**明确不做**（2026-11-11 master 决定）：

- **弹药补给**：教程里客户端会上报补给点，但 Fantasy 同样「收下不调度」（`TryHandleMapSuppliesAsync`），
  我们跟随，不影响通关
- **按关卡限定武器**：Fantasy 不限制（原样下发玩家当前穿戴），客户端数据（`*_singleplay.ini`）里也没有武器字段；
  真机确认各关卡可以正常打通
- 首登自动教程 / 老教程 `rel_Tutorial_01` / 通关奖励 / 落库：一并不做

**回归要求**：以后改动这个文件涉及的号码（405/920/107/724/710/2023/NPC 号）时，
必须跑 `TestTutorialDispatchClaimsItsOwnNumbers` 与 `TestTutorialDispatchLeavesTheOrdinaryNumbersAlone`
—— 校验的是**分发层**，只测 helper 会漏掉「闸门在 switch 之后」这类问题（本轮踩过一次）。
