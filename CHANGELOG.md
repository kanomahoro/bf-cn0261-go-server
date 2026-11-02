# Changelog

## v0.2.0

### Added

- **道具战（`mode 3`）现在可以开局。** 2015 客户端自己保有这个模式的规则
  （kr-812 `sub_53AB30` 为它建的是普通 `ModeMgr`），所以服务端只补上原版由
  服务端负责的那几件事，箱子本身仍由客户端表现：
  - **地图箱子刷新**：房主的 `C702 gcpGameReportMapData` 报告本局补给点（位置、
    箱子种类、归属队伍、刷新上下限），服务端按每个点的间隔下发 `S711
gspGameCreateGameItem`，箱子被别人吃掉后该点按原间隔补一个。报告里的
    种类不在 `ItemSpecItem` 清单内、或归属队伍不是 0/1/2 的点会被剔除并记一条 WARN。
  - **拾取校验**：`C707 gcpEatItem` 只在箱子是本局服务端创建、种类匹配、且队伍
    相符时才成立，成立后广播 `S712 gspItemEaten` + `S713 gspAddItemToSlot`；
    别队的箱子、不在清单里的种类都会被拒。
  - **使用与切槽**：`C708 gcpUseItem` 广播 `S714 gspItemUsed`；`C744
gcpSwitchItemSlot` 按 2015 客户端的单槽位回 `S747`（`result=1`，客户端保持原选择）。
  - **丢弃/投掷**：`C728 gcpReqCreateItem` 由服务端分配箱子编号并广播 `S711`，
    之后拾取同样走上面的校验。
  - **中继侧的道具包**：`32832`/`32824`/`32888`/`32894`/`32929` 不在中继白名单里，
    以前会被静默吞掉。现在只在道具战房间里转发，其中 `32832` 会按创建信封校验
    坐标与种类；当服务端已经从地图报告接管箱子时，`32832` 会被丢弃，避免同一个
    补给点出现两个箱子。
- 箱子编号从 `0x63000000` 起分配，避开角色对象（`1000000 + 100000×座位`）与
  后续的星星区间。
- 道具战的箱子种类固定为 cn-717 的 `ItemSpecItem` 十五项（101/102/108/109/115/
  120/123/127/128/130/136/137/138/139/141），`0` 表示随机箱。

### Changed

- `normalizeGameSetting` 现在放行 `mode 1` 与 `mode 3`；换成道具战后地图按
  `itemMode` 标志校验，竞技模式仍按 `normalMode` 校验。其他模式（8/13/16/101
  等）继续拒绝，直到各自的回合逻辑移植完成。
- 版本号升到 `0.2.0`，安装脚本同步到 `v0.2.0` 的发布资产。

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
