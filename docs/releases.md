# 版本记录（本地交付）

> 这份表是给**回退**用的：每个交付给 master 的包对应一个 commit 一个 tag，出问题直接
> `git checkout <tag>` 回到当时那份代码。
> 从 v0.9.19 起执行：**每交付一个包就单独 commit + 打 tag，不再 `--amend`**。
> 发布到 GitHub 的流程见 [`release.md`](release.md)，本文件只记本地交付点。

| 版本 | commit | tag | 交付包 | 状态 |
| --- | --- | --- | --- | --- |
| v1.0.0 | 见下 | `v1.0.0` | `bfserver-linux-amd64-v1.0.0.tar.gz` 等 5 个资产 | **正式发布**（11 个模式全部验收） |
| v0.9.18 | `b72db9c` 线 | `v0.9.18` | —（海盗多人收尾，已发布） | 稳定 |
| v0.9.19 | `7de1f7e` | `v0.9.19` | `bfserver-win64-v0.9.19-pveboss-20261114.zip` | 已交付（第一轮真机） |
| v0.9.19-r2 | `26ee64f` | `v0.9.19-r2` | `bfserver-win64-v0.9.19-pveboss-r2-20261114.zip` | 已交付 |
| v0.9.19-r3 | `cb4333a` | `v0.9.19-r3` | `bfserver-win64-v0.9.19-pveboss-r3-20261114.zip` | 已交付 |
| v0.9.19-r4 | `7c5d8f7` | `v0.9.19-r4` | `bfserver-win64-v0.9.19-pveboss-r4-20261114.zip` | 已交付 |
| v0.9.19-r5 | `3e3974f` | `v0.9.19-r5` | `bfserver-win64-v0.9.19-pveboss-r5-20261114.zip` | 已交付 |
| v0.9.19-r6 | `812d1ae` | `v0.9.19-r6` | `bfserver-win64-v0.9.19-pveboss-r6-20261114.zip` | **作废**（含诊断日志，master 要求只留修复） |
| v0.9.19-r7 | `1014d09` | `v0.9.19-r7` | `bfserver-win64-v0.9.19-pveboss-r7-20261114.zip` | **作废**（含中继解析器改动与日志清理，非必要） |
| v0.9.19-r8 | 见下 | `v0.9.19-r8` | `bfserver-win64-v0.9.19-pveboss-r8-20261114.zip` | 已交付（HUB 不 +1，见 r9） |
| v0.9.19-r9 | 见下 | `v0.9.19-r9` | `bfserver-win64-v0.9.19-pveboss-r9-20261114.zip` | 单人已验证（两局，一胜） |
| v0.9.19-r10 | 见下 | `v0.9.19-r10` | `bfserver-win64-v0.9.19-pveboss-r10-20261114.zip` | 双人已验证（模式 16 定版） |
| v0.9.19-r11 | `e2ed433` | `v0.9.19-r11` | `bfserver-win64-v0.9.19-pveboss-r11-20261114.zip` | **已回退**（日志分级改动未采用；tag 留档） |
| v0.9.19-r12 | 见下 | `v0.9.19-r12` | `bfserver-win64-v0.9.19-r12-20261114.zip` | 管理后台适配全部模式 |
| v0.9.19-r13 | 见下 | `v0.9.19-r13` | `bfserver-win64-v0.9.19-r13-20261114.zip` | Info 收敛（7 条降至 Debug） |
| v0.9.19-r14 | 见下 | `v0.9.19-r14` | `bfserver-win64-v0.9.19-r14-20261115.zip` | **补上地图传送点 `C948/S958`，真机已验证** |
| v0.9.19-r15 | `965b172` | `v0.9.19-r15` | `bfserver-win64-v0.9.19-r15-20261115.zip` | **已回退**（未获授权就实现，master 要求撤回；tag 留档） |

## v0.9.19 — 僵尸袭击 BOSS 战（mode 16）

功能提交：`6633be0` `feat(pveboss): the cooperative zombie raid (mode 16)`
（22 文件，+1773/−63；规则见 [`../PVEBOSS.md`](../PVEBOSS.md)）
随后 `7de1f7e` `revert(pveboss): drop the pveBoss.noInfection practice switch`
撤回练习开关（该开关是临时需求，master 决定不加；撤回后二进制与开关加入前逐字节一致）。

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-20261114.zip` | `1f728eb7473559ca31163fb8913729e8b62e1c695ada89daacd51250d3b52ada` |
| `bfserver.exe`（包内） | `39be4683b43e7e0168ef37786d616412ce153143aae42bc9cb433eb68bc5f6fe` |
| `bfserver-linux-amd64` | `8d6e82edbd70d5a37e33b5d23303d884ea69ab96112ed7f40374f1b6dd7415dc` |

**重建校验**（2026-11-14 验证过与包内 exe 逐字节一致）：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-s -w" -o bfserver.exe ./cmd/bfserver
```

## v0.9.19-r2 — 真机反馈修正

提交：`26ee64f` `fix(pveboss): map dialog, ammunition, and score-unit diagnostics`
（地图设置只显示 2 张图 → 保留请求主题；无限子弹 ⇒ 撤回 ammoSupplyMode(16)；
PVE 积分诊断：分数单位前 5 条 Info + 结算计数 + 未认领房级包打印）

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r2-20261114.zip` | `544d080c206f433fabd425052bbba9f50cc0471fa980620a6825dbf729aa9c3a` |
| `bfserver.exe`（包内） | `c6469b25da6718ecd0c7fd38ae5f2138f76af7a5bd1d94db7fb8c64c85b93c41` |
| `bfserver-linux-amd64` | `ec0d2bf29c0e0ae2c6c9d2f82f660ae74c4a03c1c965c4aed6ef8e2b2b11fa93` |

## v0.9.19-r3 — 实时计分表回显

提交：`cb4333a` `fix(pveboss): echo the live score table the client asks for`
（`C710`/`C2023` → `S729`；真机日志里客户端 2 分钟发了 140 次 C710，我们全丢了）

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r3-20261114.zip` | `9e21e4a4ad347d1c0283090c81f986793bf2b23ce189358ddf4239eb2a1251fc` |
| `bfserver.exe`（包内） | `038407a7987b5b7f929ce95798df98843001018551db8d20f26d9456460f8914` |
| `bfserver-linux-amd64` | `ea245f6cb7bf606c9fcc6e4d71b5cc90d9264498739632d822b72febb2c2abcb` |

## v0.9.19-r4 — 掉落与拾取（C728 → S711 → C707 → S712 + 715）

提交：`7c5d8f7` `feat(pveboss): materialise the loot a mob drops`
（真机日志里一局 44 次 C728，我们全丢 → 怪死了什么都不掉；HUB 的计次靠拾取）

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r4-20261114.zip` | `5d76e170558ab1e38b56e79a5e32a703623a8c0ec72647f0bb80cefaef55c4ad` |
| `bfserver.exe`（包内） | `36d07c7a2ce282ec0757eeba3da7eec1a858ed0168436abff94ad0e6b17fd928` |
| `bfserver-linux-amd64` | `6a7a86fb1215aa7910b7cf3ad8302302a2fdad897925e5feb492a168f7e768ec` |

## v1.0.0 — 第一个正式版本

`v0.9.19` 之后累积的修复与收尾（r2~r14）全部合入；r11 / r15 两版已回退，仅留 tag 存档。
改动范围：模式 16 定版（r2~r10）、地图传送点（r14）、管理后台模式适配（r12）、Info 日志收敛（r13）＋文档。

| 资产 | sha256 |
| --- | --- |
| `bfserver-linux-amd64-v1.0.0.tar.gz` | 见 `SHA256SUMS`（随 Release 上传） |
| `bfserver-linux-arm64-v1.0.0.tar.gz` | 同上 |
| `bfserver-android-arm64-v1.0.0.tar.gz` | 同上 |
| `bfserver-windows-amd64-v1.0.0.zip` | 同上 |
| `bfserver-windows-arm64-v1.0.0.zip` | 同上 |

## v0.9.19-r14 — 补上地图传送点：`C948 gcpPortalMove` → `S958 gspPortalMove`

按 fantasy 的 `packets/portal.yaml` + `RoundFlow.Portal.cs` 对 cn-717 的实现补齐（此前仓库里 948/958 零命中，
地图上的传送点踩了没反应）。

| 方向 | 包 | 长度 | 结构 |
| --- | --- | --- | --- |
| C2S | `948 gcpPortalMove` | 14 | `x f32` `y f32` `z f32` `portal_id u16` |
| S2C | `958 gspPortalMove` | 10 | `unknown_4 u32`(0 放行/−1 不移动) `portal_id u16` `seat u32` |

行为与参考实现一致：只校验「正在对局中 + session 拥有该座位 + 坐标有限」，然后
**广播给全房（含发起者）**；不校验区域号存在性、不接管位置权威。中继伴生帧
`rcpPortalMove`(26)/`rcpPortalMoveAfter`(27) 按参考实现的结论**不授予移动**，只认游戏连接那条。

- 新文件：`internal/protocol/portal.go`、`internal/game/portal.go`
- 分派：`internal/game/server.go` 新增 `case 948`
- 测试：`TestPortalMoveRoundTrip`（与参考实现同一段 hex，回复必须是 `00000000341205000000`）、
  `TestPortalMoveRejectsBadBodies`（截断/补齐/NaN 全拒）、`TestPortalMoveReachesEverySeat`、
  `TestPortalMoveOutsidePlayIsIgnored`
- 文档：`docs/design.md` 新增「地图传送点（portal）」一节

**真机结论（2026-11-14 23:38）**：传送点正常，单人落地后可正常传送。

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-r14-20261115.zip` | `fa0bdb1d00986ca8d931f9e816f1128920b6b447cb6783880ff2f1d800dcc132` |
| `bfserver.exe`（包内） | `30929d07f66fceaba23ba2609dd5c7f47bd9dbaa957d33a2dbad49cbd25ec9ae` |
| `bfserver-linux-amd64` | `6ce56b7078c2346c916cea9afbb8baa81cc8efe264aa5fb812278bd74a7425b6` |

## v0.9.19-r13 — Info 里移走 7 条过程细节（不新增开关）

按真实日志（`Server (16)`，两局 9 分钟）核对了 Info 的实际条数，把过程细节降到 Debug：

| 行 | 该局条数 | 为什么降级 |
| --- | --- | --- |
| `[PveBoss] unclaimed room packet` | **60** | 诊断用，每局封顶 30 条，仍是最占量的 Info |
| `[PveBoss] score unit accepted` | 10 | 每局前 5 条，同数据已有 `[PveBoss] points`（Debug） |
| `[PveBoss] loot box created` | 10 | 每局前 5 条，拾取侧另有 `ammo box taken` |
| `[PveBoss] score unit ignored` | 0 | 同上，属每次上报的细节 |
| `[PveBoss] the raid starts` | 0 | 与 `the round starts`（已含 seats）重复，后者保留 Info |
| `[PveBoss] a zombie seat came back` | 0 | 客户端每发一次复活请求就一条，可被重复触发 |
| `[Pirate] spawn group from the host` | 0 | 每个刷怪组一条，含 hex |

保留在 Info 的仍是「一局个位数」的里程碑：`[Round] started/ended/cancelled`、`[PveBoss] the round starts /
mission started·cleared·timed out·failed / seat turned / nobody is left / stage reported / the boss phase is due /
the map ships no mission groups`、`[Pirate] wave / wave table / every wave cleared / stage reported`、
`[Supplies] map report`。

没有新增配置项：只把级别从 Info 改成 Debug。移走后同一局 Info 由 80 余条降到约 25 条。

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-r13-20261114.zip` | `5bc249334a048dd3b1cce9ff57337fd41d9afb48d0fa93eda5f665ab5e2c1046` |
| `bfserver.exe`（包内） | `df1b156920e06c3cef33653a0eed1ce2dbdca71b3473631f10c5d18dd300e08c` |
| `bfserver-linux-amd64` | `7c10f22a8d1c7cbd3b2ee56002053e28ee7ca3f59bb035bf58d5dc8dd40e6bd3` |

## v0.9.19-r12 — 管理后台补齐全部模式

一直加模式却没回头看后台，核对后发现三处缺口（现由 `internal/admin/modes_test.go` 钉住）：

| 问题 | 现象 |
| --- | --- |
| 模式名表缺 **19 爆破 / 20 队长战** | 房间列表与战绩列表把这两类显示成「模式 19」「模式 20」 |
| 战绩筛选下拉缺 **16 / 19 / 20 / 101** | 无法按僵尸BOSS、爆破、队长战、海盗筛选对局 |
| 合作模式「胜方」显示红/蓝 | 16 与 101 没有红蓝双方，且两套约定相反（海盗 `1`=玩家通关，僵尸BOSS `2`=玩家通关） |

改动：

- `internal/admin/index.html`：模式表补齐 11 个模式（16 改名「僵尸BOSS」），筛选项补齐；
  新增 `COOP_WIN`，16/101 的胜方列显示「通关 / 失败」
- `internal/admin/modes.go`（新）：CSV 用的模式名表（原先只有数字）
- `internal/admin/modes_test.go`（新）：Go 表、页面 `MODE`、筛选下拉三者必须一致
- CSV 新增 `mode_name` 列
- 文档：`docs/admin.md` 新增「模式名」一节（含两套合作模式胜方约定）；README 的模式表把 16 改成
  「单人+双人真机通过 · 定版」

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-r12-20261114.zip` | `1a2088be54daab57dcb04071c5f04aea8189afdb9c4155dd48b41e560fec1c8c` |
| `bfserver.exe`（包内） | `306da274a6e9f22b20b26f37dc7422c3a5915e175cfbc8b3b49c635cb48314a3` |
| `bfserver-linux-amd64` | `4361f764ca4d7c9ef06460ac485c152686a09496e0f5964d7e306d5640d3c200` |

游玩逻辑零改动（只动管理后台页面、CSV 与一个只读的模式名表）。

## v0.9.19-r10 — 双人反馈的两处死亡/复活缺陷

双人真机（`Server (15)`）报了两个问题，根因都在死亡与复活这条路上：

### 1. 非房主 0 血不死、像无敌

**现象**：第一局玩家 2（非房主）血条到 0 却继续行动，房主看他一直活着。

**日志**：整局 `slot=1` 发了 **207 次 `C925 gcpPveUseRevival`**（一次 ≈1.5 s），却**没有一条死亡事件**
（`32836 kind=5 victimState=dead` 全程只有 2 条，都是房主自己的）。我们的转换只认死亡事件，而
`C925` 从人类座位发出时被直接忽略 → 服务端一直认为他还活着，客户端停在「0 血待复活」状态。

**修法**：raid 里 `C925` 来自人类座位 = 该客户端在报「0 血要复活」，直接按死亡处理（转僵尸）。
参考实现同一份注释也写着「modes 16 and 101 在 0 血发 gcpPveUseRevival」。

### 2. 房主死后变僵尸，站起来又躺下、读人类复活条、卡死

**现象**：房主死亡 → 立刻变僵尸（站起来）→ 随即又躺下 → 出现**人类**复活读条 → 读条走完仍躺着，
无法操作。玩家 2 被爆炸炸死立即转阵营，随后整局失败。

**日志**：房主 0 血时先发 `C925`，148 ms 后又发 `32904 gcpGameReqRespawn`。我们对 `32904` 走的是
**通用复活流程**（`RequestRespawn` + 延迟后发「战点复活」的 spawn offer），而僵尸座位在 raid 里
没有对应的战斗出生点 → 客户端拿到一个用不上的复活点 → 复活条走完也站不起来。

**修法**：raid 里已转阵营（team 15）的座位请求复活时，**不再走通用复活**，改为重发一次
team-15 通知（参考实现就是这样：僵尸靠「重复的队伍通知」自己站起来）。

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r10-20261114.zip` | `0563e1e3509e09c427c94402a865ad17c542650fc561b6e7d00feaf660514098` |
| `bfserver.exe`（包内） | `3a544b28d69f176d4ccd7a6f097983dd3c1cb1002fc97e487e87171699a87ae0` |
| `bfserver-linux-amd64` | `74a42272b9ec69e8420acf69378095a63628e2f363526d7c613999a5addf66cc` |

代码改动：`pveBossRevival`（`internal/game/pveboss.go`）+ `handleRespawn` 的 raid 分支
（`internal/game/battle.go`），两条都在 `room.PveBossRound()` 守卫内，老模式零改动。
18 个用例全绿（新增 `TestPveBossZeroHpRevivalTurnsTheSeat`、`TestPveBossZombieRespawnRepeatsTheNotice`）。

## v0.9.19-r9 — 补 `S713 gspAddItemToSlot`：计数道具要「授予」才进 HUB

真机（r8）：捡到 229/146/136 **不再自动使用**（对），但**道具直接消失、HUB 不 +1**。

**从参考实现抄到的答案**（`RoundFlow.Supplies.cs` 的 `HandleSupplyPickupAsync` +
`Dialect.Rooms/SupplyBinding.cs` 的 `RoundSupplyPicked` 发送器）：一次拾取要发**两个**包：

1. `S712 gspItemEaten`（seat, object_id）—— 所有客户端把箱子删掉
2. 若 `ItemKind > 0`：`S713 gspAddItemToSlot`（seat, kind, item_slot）—— 这才是「你现在持有这件道具」的确认

其中计数类（counted）**item_slot 固定 0**：参考实现里 `slotted && !counted` 才分配原生槽位，
计数类不进原生槽，只让客户端的 HUB 计数 +1。即时类则是 `ItemKind = 0` → 不发 713，改发 `S715 gspPveItemUsed`。

r8 只做了「不回 715」，漏了 713 → 客户端收到 S712 把箱子删了，却没人告诉它「你拿到了」 → 消失且不计数。

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r9-20261114.zip` | `f6c48995b887da4c6cc1bf34c96d802235e3a69b6c38cc34fbe29bf5ca7d2803` |
| `bfserver.exe`（包内） | `d07dcea2e1ca9ed307d809f8046994fb0e7faf7a6def8fc6f45d2060a73eb322` |
| `bfserver-linux-amd64` | `df15480a17ed8257e0a45081d19791328196669d6a0da391772ba74dc26bd8bc` |

相对 r8 的唯一改动：`takeAmmoBox` 里计数类改成 `room.Broadcast(S713(seat, kind, 0))`（不再发 715）；
即时类与非 raid 模式仍是 715。16 个用例全绿（新增 `TestPveBossCountedLootIsGranted`）。

## v0.9.19-r8 — 只保留修复：HUB 道具拾取后计次（相对 r5 唯一改动）

master 2026-11-14 18:15 决定：**从 r5 起只保留真正修复问题的改动，其余全部回退**。
因此本版 = `v0.9.19-r5` + **一个判断**，代码差异仅 `+18/−1`（2 文件），且都在 raid 守卫内：

```go
// internal/game/supplies.go — takeAmmoBox
if !room.PveBossRound() || !pveCountedKind(boxKind) {
    _ = session.Send(protocol.CreatePveItemUsedReply(uint32(seat), boxKind))
}
```

- raid 房里拾到 **229/146/136**（HUB 三槽）不再回 `715 gspPveItemUsed` → 只广播 `S712`，客户端计次 +1，玩家自己按键使用
- **101/17**（加速/补给手雷）与**所有非 raid 模式**仍照旧回 `715`
- 回退掉的非修复项（r6/r7 里曾有）：`loot taken` 诊断日志与计数器、`knownCarriedKind` 的 PvE 白名单、
  中继 `32773` 解析器改动、raid 里 `32813` 外部对象忽略

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r8-20261114.zip` | `fd5a907e378f4cead809098f1e581a95a09bcb85c8437562ee508d4bf554e9df` |
| `bfserver.exe`（包内） | `f60f54b1344b83e6b6db89265bcdd1b0e47eed528ddc63b652e3497e8114cb31` |
| `bfserver-linux-amd64` | `28fa60b783d50eddc212f62fa055f9af825bd8d7a33c86597a145bc2bec817c8` |

**这一版的单元测试**：15 个用例全绿（r5 的用例集，未新增；`_test.go` 不进库）。

## v0.9.19-r5 — 撤回计分表回显 + 掉落

提交：`3e3974f` `revert(pveboss): drop the score-table echo`
（实时计分走 S986，海盗模式历来如此；C710 只认领不回包）+ r4 的掉落链路（`7c5d8f7`）

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-r5-20261114.zip` | `6a223a98f1006281f4348bdaf3ced6ef05f30750f2e44f08076149e2cb0cabea` |
| `bfserver.exe`（包内） | `2f665ed334cbaecc5987d83889e8e04fd633646b69af01e10c26c66fe710ef6a` |
| `bfserver-linux-amd64` | `8a4d6fb21d1f3997bb1d886a0fe9430f996a5e79913a90c8e7092b494c190838` |

**这一版的单元测试**（14 个，`*_test.go` 不进版本库）留档在
`~/.openclaw/workspace/artifacts/2026-11-14-pveboss/`。
