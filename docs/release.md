# 发布流程（打 tag 与上传资产）

> **版本号策略**：出号由 master 决定（当前已出到 **0.9.0**）；中间的修复与模式改动先累积，出号时把它们归并到
> 一个 `## vX.Y.Z` 段里。下面的流程是**发版时**才走一遍的。
>
> **版本库不含单元测试**：`*_test.go` 在 `.gitignore` 里，发布树也不带；回归时从历史取回
> （`git log --all --diff-filter=D -- '*_test.go'`）。

出号时按顺序做，四步都要对上，否则一键脚本会 404：

```bash
# 1. 改版本：cmd/bfserver/main.go 的默认 Version、README badge 与正文、tools/bfserver-install.sh 的 VERSION
#    ⚠️ 用 `-buildvcs=false` 构建：Go 默认会把 git revision 与「工作区是否被改过」写进二进制，
#    那样改一行文档哈希就变，BIN_SHA256 永远对不上。加了这个开关后产物只由源码与 flag 决定。
# 2. 构建两端产物（Linux 供一键脚本，Windows 供手动测试）
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w" -o bfserver ./cmd/bfserver
#    版本号改在源码里（cmd/bfserver/main.go 的 Version），不要再加 -X main.Version 覆盖
#    用固定 mtime/属主打包，tar.gz 才可复现（否则每次都不同，跟 Go 无关）
tar --sort=name --mtime='UTC 2026-01-01' --owner=0 --group=0 --numeric-owner \
    -czf bfserver-linux-amd64-vX.Y.Z.tar.gz bfserver
# 3. 算 tar.gz 的 sha256，写回 tools/bfserver-install.sh 的 BIN_SHA256
sha256sum bfserver-linux-amd64-vX.Y.Z.tar.gz
# 4. 提交、打 tag、推 main 与 tag，然后在 GitHub 建 Release 并上传资产
#    Release 只放二进制包 + SHA256SUMS，**不传 bfserver-install.sh**：
#    脚本永远从仓库取（raw 的 main 或某个 tag），不跟着 Release 发，也就不存在两份脚本不一致的问题。
#    一键脚本靠 tag 里的自己去找 Release 资产，所以 tag 必须先推、资产必须先传。
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin main
git push origin vX.Y.Z
```

Release 页要传的资产（六个）：

```
bfserver-linux-amd64-vX.Y.Z.tar.gz
bfserver-linux-arm64-vX.Y.Z.tar.gz
bfserver-android-arm64-vX.Y.Z.tar.gz
bfserver-windows-amd64-vX.Y.Z.zip
bfserver-windows-arm64-vX.Y.Z.zip
SHA256SUMS
```

`SHA256SUMS` 只记上面五个包，不把脚本或文档算进去。
