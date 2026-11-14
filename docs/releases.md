# 版本记录（本地交付）

> 这份表是给**回退**用的：每个交付给 master 的包对应一个 commit 一个 tag，出问题直接
> `git checkout <tag>` 回到当时那份代码。
> 从 v0.9.19 起执行：**每交付一个包就单独 commit + 打 tag，不再 `--amend`**。
> 发布到 GitHub 的流程见 [`release.md`](release.md)，本文件只记本地交付点。

| 版本 | commit | tag | 交付包 | 状态 |
| --- | --- | --- | --- | --- |
| v0.9.18 | `b72db9c` 线 | `v0.9.18` | —（海盗多人收尾，已发布） | 稳定 |
| v0.9.19 | `7de1f7e` | `v0.9.19` | `bfserver-win64-v0.9.19-pveboss-20261114.zip` | 已交付（第一轮真机） |
| v0.9.19-r2 | `26ee64f` | `v0.9.19-r2` | `bfserver-win64-v0.9.19-pveboss-r2-20261114.zip` | **已交付，待第二轮真机** |

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

**这一版的单元测试**（14 个，`*_test.go` 不进版本库）留档在
`~/.openclaw/workspace/artifacts/2026-11-14-pveboss/`。
