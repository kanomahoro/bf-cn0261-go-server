# 文档索引

按「怎么用 / 怎么改 / 每个模式」三类。改动代码前先看对应模式文档，所有包号结论都写在里面。

## 1. 使用与部署

| 文档 | 内容 |
| --- | --- |
| [`../README.md`](../README.md) | 项目总览、**模式一览（含加入版本）**、快速启动、部署与排障 |
| [`manual-install.md`](manual-install.md) | 手动安装、登录器与客户端连接步骤 |
| [`configuration.md`](configuration.md) | `config/server.json` 每个字段的含义（端口、relay、日志、认证） |
| [`database.md`](database.md) | 表结构、迁移脚本、结算落库的字段来源 |
| [`admin.md`](admin.md) | 管理后台（房间、封禁、在线玩家、日志查询） |
| [`release.md`](release.md) | 发布流程：编译参数（`-s -w`）、打 tag、上传资产、版本号冻结策略 |
| [`releases.md`](releases.md) | **本地交付记录**：每个交付包对应的 commit / tag / sha256，回退用 |

> **打包参数固定**：`CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w"`。
> 漏掉 `-s -w` 会让 exe 从 ~9.3 MB 涨到 ~13.3 MB（符号表 + DWARF）。

## 2. 架构与实现

| 文档 | 内容 |
| --- | --- |
| [`design.md`](design.md) | 连接、会话、房间、回合的生命周期与并发模型 |
| [`logging.md`](logging.md) | **日志级别约定**：哪些事件走 Info、哪些只能 Debug，以及各模式的分类表 |
| [`../CHANGELOG.md`](../CHANGELOG.md) | **每个版本改了什么、为什么改**（含现象、根因、真机日志），最新在最上面 |

## 3. 模式文档

每个模式一份，写规则、协议号、状态机、真机踩过的坑。

| 文档 | 模式 | 状态 |
| --- | --- | --- |
| [`../ZOMBIE.md`](../ZOMBIE.md) | 8 僵尸（选王，180s） | 已完成，真机通过 |
| [`../CAPTAIN.md`](../CAPTAIN.md) | 20 队长战 | 已完成，真机通过 |
| [`../BOMB.md`](../BOMB.md) | 19 爆破（三局两胜） | **已定稿**：三局流程/结算/弹药箱通过；结果横幅与 HUD 队伍比分不再尝试 |
| [`../TUTORIAL.md`](../TUTORIAL.md) | 教程（单人，25 关） | 已完成，真机通过；弹药补给与武器限制不做 |
| [`../PIRATE.md`](../PIRATE.md) | 101 海盗（1–4 人合作） | **单人+双人真机通过**：波次/同步/结算/计分（泡封·击爆·营救）均已验收；不发奖励，掉落只消费不执行 |
| [`../PVEBOSS.md`](../PVEBOSS.md) | 16 僵尸袭击 BOSS 战（1–4 人合作） | **单人+双人真机通过 · 定版**（`v0.9.19-r14`）：救援任务闹钟、PVE 点数/排行、三格计数道具、感染转队、结算不发奖励 |

模式 1 / 3 / 12 / 13 / 14 / 18 没有独立文档：它们没有专属状态机，规则写在 README 的「模式一览」与
CHANGELOG 对应版本里（12/18 的强制狙击枪、13 的无队伍结算、3 的 15 种道具箱都在那儿）。

客户端可选模式共 11 个，**已全部实现**：1 / 3 / 8 / 12 / 13 / 14 / 18 / 19 / 20 / 101 / 16，外加单人教程（25 关）。
16 僵尸 BOSS 战是最后一个，已随 v0.9.19 落地，单人+双人真机通过（定版 `v0.9.19-r14`，含地图传送点）。

## 4. 调研与素材（在 agent workspace，不在本仓库）

| 路径 | 内容 |
| --- | --- |
| `workspace/notes/bf-tutorial-mode-research.md` | 教程模式的调研过程与证据 |
| `workspace/notes/data-bf-tutorial-stages.csv` | 25 关的 MapMeta 数据 |
| `workspace/artifacts/2026-11-04-clientdata/` | 国服 2015 客户端数据解包件（MapMeta、Config.bfz 工具等，**别删**） |
| `workspace/artifacts/logs/2026-11-11/` | 真机日志归档 + 每份日志证明了什么 |

## 5. 写作约定

- 包号一律以 **我们自己的客户端**（CN0261 / 协议 717）为准；只查 `profiles/cn-717/opcodes.yaml` 这类 verified 表，
  `opcode-catalog.yaml` 的 `conflict` 条目只能当线索（曾把计数包号写成 711，正确是 729）
- 同号不同义要显式写出来（我们 S702 = everybody-loaded，cn-279 表里 702 = map item data）
- 真机结论要写「日期 + 日志文件 + 结论」，方便以后回查
- 新增功能时**三处一起改**：README 的模式一览、CHANGELOG 的版本段、对应模式的 `<MODE>.md`（如果有）
- 修复不往 README 里堆：README 只放模式与里程碑，逐条修复（现象/根因/证据）一律进 CHANGELOG
