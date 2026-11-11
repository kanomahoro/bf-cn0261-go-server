# 文档索引

按「怎么用 / 怎么改 / 每个模式」三类。改动代码前先看对应模式文档，所有包号结论都写在里面。

## 1. 使用与部署

| 文档 | 内容 |
| --- | --- |
| [`../README.md`](../README.md) | 项目总览、快速启动、已支持的模式与协议版本 |
| [`manual-install.md`](manual-install.md) | 手动安装、登录器与客户端连接步骤 |
| [`configuration.md`](configuration.md) | `config/server.json` 每个字段的含义（端口、relay、日志、认证） |
| [`database.md`](database.md) | 表结构、迁移脚本、结算落库的字段来源 |
| [`admin.md`](admin.md) | 管理后台（房间、封禁、在线玩家、日志查询） |
| [`release.md`](release.md) | 发布流程：编译参数（`-s -w`）、打 tag、上传资产 |

> **打包参数固定**：`CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w"`。
> 漏掉 `-s -w` 会让 exe 从 ~9.3 MB 涨到 ~13.3 MB（符号表 + DWARF）。

## 2. 架构与实现

| 文档 | 内容 |
| --- | --- |
| [`design.md`](design.md) | 连接、会话、房间、回合的生命周期与并发模型 |
| [`logging.md`](logging.md) | **日志级别约定**：哪些事件走 Info、哪些只能 Debug，以及各模式的分类表 |
| [`../CHANGELOG.md`](../CHANGELOG.md) | 每个版本的改动与真机结论（**最新在最上面**） |

## 3. 模式文档

每个模式一份，写规则、协议号、状态机、真机踩过的坑。

| 文档 | 模式 | 状态 |
| --- | --- | --- |
| [`../ZOMBIE.md`](../ZOMBIE.md) | 8 僵尸（选王，180s） | 已完成，真机通过 |
| [`../CAPTAIN.md`](../CAPTAIN.md) | 20 队长战 | 已完成，真机通过 |
| [`../BOMB.md`](../BOMB.md) | 19 爆破（三局两胜） | **已定稿**：三局流程/结算/弹药箱通过；结果横幅与 HUD 队伍比分不再尝试 |
| [`../TUTORIAL.md`](../TUTORIAL.md) | 教程（单人，25 关） | 已完成，真机通过；弹药补给与武器限制不做 |

客户端可选模式共 11 个，已做完 9 个：1 / 3 / 8 / 12 / 13 / 14 / 18 / 19 / 20 + 教程。
剩 **16 僵尸BOSS**、**101 海盗**。

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
