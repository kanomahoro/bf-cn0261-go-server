# 爆破模式（mode 19）

客户端 `wgtBomb`，地图标记是 `MapMeta.xml` 的 `isBombmode`。**2026-11-10 实现完成，待真机验收**。
规则、协议证据、子局重开机制与仍需真机确认的边角都在本文。

## 规则：一个系列，三局两胜

原版是**整场**比赛，不是单局：三局两胜（`rounds_to_win = 2`），**每一小局攻守互换**，
每小局结束显示「本局是进攻方/防守方胜利」，**三局打完才结算整场**（一次 S729 落库）。

| 项 | 值 |
| --- | --- |
| 小局数 | 0 基 0..2，`attacking = 局号 % 2 + 1`（第 0 局 1 队攻，第 1 局 2 队攻） |
| 小局时长 | 地图 240 s |
| 炸弹倒计时 | 60 s（客户端原生，服务端不参与） |
| 复活 | **无**。死亡即观察者，直到本小局结束 |
| 小局胜负 | 炸弹爆 → 进攻方；拆弹成功 → 防守方；时间到未爆 → **防守方**（已装弹则时间到本局不结束） |
| 整场胜负 | 先赢两局的一方 |

## 协议

| 包 | 号 | 方向 | 体 | 说明 |
| --- | --- | --- | --- | --- |
| `gspBombState` | **973** | S2C | 24 B：operator / state / x,y,z / timer_aligned | state 0..6 |
| `gcpBombState` | **2012** | C2S | 20 B：五个 word | 客户端上报 |
| `gcpBombMissionOver` | **2011** | C2S | 12 B：winning_team / planter / defuser | 只认**房主** |
| `gspObserver` | **974** | S2C | 4 B：actor | 转身为观察者 |
| `gppMobDropBombOnDead` | 32877 / 32874 | — | 41 B | 小怪掉弹，**未实现** |

- `operator` 是客户端自己的 **user number**（`UserSlot+8` → `Character+756`），**不是场景对象号**。
  服务端不解析它：上报者由连接确定，收发都原样透传，只额外记住「某座位第一次报的号」，
  之后该座位换号即拒绝（防止冒用别人的 operator）
- `gspObserver` 的 `actor` 才是**场景对象号** = `1000000 + seat × 100000`（我们的 `objectFor`）
- `timer_aligned` 一律发 `false` → 客户端跑自己的 60000 ms 倒计时（参考实现同样留 false）

### 状态机（照抄 kr-812 `sub_4245B0` / 参考实现 `BombRoundState`）

| 目标状态 | 允许的前置 | 谁能报 |
| --- | --- | --- |
| 0 reset | 装机中 | 装机者本人 |
| 1 装机 | 0 | 本人、**进攻方**、站着 |
| 2 已装（60 s） | 1 | 装机者本人、可操作 |
| 3 爆炸 | 2 或 4 | **房主**，且 operator == 装机者 |
| 4 拆弹中 | 2 | 本人、**防守方**、站着 |
| 5 拆包中断 | 4 | 拆弹者本人（回 2，不倒计时重来） |
| 6 拆弹完成 | 4 | 拆弹者本人（清 planter 与坐标） |

- **坐标只在装机时写入**，之后每条上报必须与装机坐标**完全相等**（浮点精确比较），否则丢弃
- 操作者离房 → 广播取消：装机中 → 回 0；拆弹中 → 回 5（且不开新倒计时）

### 2011 的校验（`bombMatchesResult`）

| 当前炸弹状态 | 合法的上报 |
| --- | --- |
| 3 爆炸 | 赢方 = 进攻方，planter = 装机者，defuser = -1 |
| 6 拆弹完成 | 赢方 = 防守方，planter = -1，defuser = 拆弹者 |
| 2/4 已装 | 赢方 = 进攻方，planter = 装机者，defuser = -1，且**防守方全灭** |
| 0/1 未装 | planter = defuser = -1，且对方**全灭**（客户端房主自己判） |

「全灭」的判定含被泡封可救的队友（参考实现的 `Alive or Bubble`）。

## 系列数据怎么传

三个地方，都是**我们已经有的包**，只是原先填 0：

| 位置 | 字段 | 值 |
| --- | --- | --- |
| S510 `unknown_36` | 小局号 | 0..2（字符串之后第一个 word） |
| S510 `unknown_40` | 进攻方 | 1 / 2 |
| S510 `unknown_44` | roomState | 3（`MapRoomState`，与所有模式一致） |
| `GameSetting +172`（`Values148[6]`） | settings flags | 第 0 局置 bit0，其后清掉 |
| `GameSetting +176`（`Values148[7]`） | 小局号 | 0..2 |

## 子局重开（最容易翻车的地方）

一小局结束后**不重进房间**：只把客户端重新带回一次「加载 → 倒计时 → 开打」。

1. 小局结束 → 逐人发 S705（3 胜 / 2 负）显示本局攻守方胜利 → 停表、清炸弹、`phase = settling`
2. 等 `bombRoundPause`（5 s，让横幅播完）
3. 局号 +1、攻守互换、炸弹重置、写回 setting 的两个系列字段、`generation++`、`phase = loading`
4. 重新 `bindLife`（小局分数清零）、重发入场包（S503 + 每个座位 S503/S505 + S522 + S510(系列) + peer 三件套 + **同一个 relay key**）
5. 重新武装加载门：等客户端各自上报 C701 → 现有 `handleMapLoaded` → 倒计时 → `startRound`

relay key 在整场里**保持不变**（重开时不重新注册），否则客户端的 relay 通道会失效。

## 结算

- 小局结束**不发** S729、不落库、不发奖励
- 整场结束（一方两胜）才走一次完整结算：`finishRound` 的 `decided` 链里新增 `seriesWinner`，
  优先于比分比较
- 掉线弃权：一侧清空 → `forfeitWinner` → 整场结束（与参考实现的 `TryForfeit` 一致）
- 比分上限**不结束小局**（爆破由目标决定，杀人数不判负）

## 实现位置

| 文件 | 内容 |
| --- | --- |
| `internal/protocol/bomb.go` | 973/2011/2012/974 的编解码、S510 系列变体、状态名 |
| `internal/game/bomb.go` | 系列状态、状态机校验、观察者、小局结束与推进、重开 |
| `internal/world/bombmaps.go` | 6 张爆破图（含 2 张只属于爆破的图） |
| `internal/game/rooms.go` | 入场序列抽成 `broadcastRoundEntry`、`finishRound` 一行 `seriesWinner`、`cancelRound` 复位、`roundClockExpired` |
| `internal/game/battle.go` | 爆破局把复活请求改成广播 S974 |
| `internal/game/lifeplay.go` | 爆破局比分上限不结束小局 |

## 地图池（6 张，来自 `MapMeta.xml` 的 `isBombmode`）

`rel_Village03_CityHall`（Village 3）、`rel_Desert02_Castle`（Desert 4）、`rel_Forest02_Ravine`（Forest 3）、
`rel_Village02_waterPark`（Village 4）、`rel_zombie_01_bomb`（Village 5）、`rel_Sudden06_Tank`（Sudden 4）。

后两张 `isNotemTeamModeMap = 0`，不在竞技图池里，因此**只存在于爆破池**（`world.bombMapPool`）。
爆破图池与 `game_maps.json` 完全隔离：竞技模式的候选图不会漏出这两张。

## 待真机确认

1. **子局重开**：客户端在重发的 S510 之后是否重新上报 C701（参考实现会 arm 加载超时，说明会）。
   若不重报，退化为「跳过加载门，直接倒计时」
2. 本局横幅的时长是否够 5 s（`bombRoundPause` 可调）
3. 观察者：是否真的只有死者需要，还是需要把「全队阵亡」也提前告知
4. 时间到未爆 → 防守方赢已在方案上对齐（参考实现一致），真机看提示文字是否对
5. S973 之后客户端是否需要额外的「炸弹位置」包（爆炸特效使用 32xxx 特效包，暂由通用转发处理）
