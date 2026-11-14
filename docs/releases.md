# 版本记录（本地交付）

> 这份表是给**回退**用的：每个交付给 master 的包对应一个 commit 一个 tag，出问题直接
> `git checkout <tag>` 回到当时那份代码。
> 从 v0.9.19 起执行：**每交付一个包就单独 commit + 打 tag，不再 `--amend`**。
> 发布到 GitHub 的流程见 [`release.md`](release.md)，本文件只记本地交付点。

| 版本 | commit | tag | 交付包 | 状态 |
| --- | --- | --- | --- | --- |
| v0.9.18 | `b72db9c` 线 | `v0.9.18` | —（海盗多人收尾，已发布） | 稳定 |
| v0.9.19 | `6633be0` | `v0.9.19` | `bfserver-win64-v0.9.19-pveboss-20261114.zip` | **已交付，待真机** |

## v0.9.19 — 僵尸袭击 BOSS 战（mode 16）

功能提交：`6633be0` `feat(pveboss): the cooperative zombie raid (mode 16)`
（22 文件，+1773/−63；规则见 [`../PVEBOSS.md`](../PVEBOSS.md)）

| 资产 | sha256 |
| --- | --- |
| `bfserver-win64-v0.9.19-pveboss-20261114.zip` | `185bca06bbee44ce34da6c1befcde304d9ae715529a69a3fc65702d008138535` |
| `bfserver.exe`（包内） | `182e2262387be49f8e3385131cad59bbd634facb04c225adbea3bb9043fd96fb` |
| `bfserver-linux-amd64` | `671bf2e5079829f99b44e274072782083470506986744f73ce598a9acb902fbb` |

**重建校验**（2026-11-14 验证过与包内 exe 逐字节一致）：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-s -w" -o bfserver.exe ./cmd/bfserver
```

**这一版的单元测试**（15 个，`*_test.go` 不进版本库）留档在
`~/.openclaw/workspace/artifacts/2026-11-14-pveboss/`。
