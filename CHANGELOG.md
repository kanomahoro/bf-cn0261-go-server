# Changelog

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
    之后按各自间隔补；`itemMode.disableMapReportSpawns` 用来关掉它（当某个版本
    确认自己会放点位箱子时）。
  - **未知箱子也认**：既然箱子由客户端创建，服务端没登记过编号的拾取请求同样
    成立（种类要在 `ItemSpecItem` 清单内，两个道具槽仍然限制持有量），否则一旦
    客户端换了创建包的时机，整局道具就全捡不起来。
  - **颜色规则**：红/蓝跟队伍**固定绑定**，别队的箱子客户端自己就不让捡
    （kr-812 `sub_6378A0`）。服务端按 `32832` 里的颜色字再核一遍：默认只记
    WARN，把 `itemMode.enforceBoxColour` 打开后改为硬拒（颜色字编号需先在真机
    确认，映射错了会让玩家连自己的箱子都捡不起来）。
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
    窗口之外的随机箱请求拒绝（`itemMode.acceptRandomBoxDrops` 可以放开）。
    真正把手持道具扔出去（`kind` 非 0）始终允许，并且会清掉对应槽位。
  - **4 种"拾取即生效"的道具**（10/11/15/16）不占槽位，单独处理。
  - **`S714` 必须照发**：曾经推断"客户端自己会通知其他人，服务端不必发"，结果是
    一整局道具全都按不出来——使用方客户端要靠这个包才应用效果。已改回始终广播。
  - **内容物种类不再做白名单**：客户端自己决定箱子里是什么，而且它拣起箱子时本地
    就把箱子删了，服务端按种类拒绝只会让"箱子消失、手里什么都没有"。现在任何种类
    都放行，只在遇到本服务端不认识的种类时记一条日志。
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
