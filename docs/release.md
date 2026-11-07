# 发布流程（打 tag 与上传资产）

改版本号时按顺序做，四步都要对上，否则一键脚本会 404：

```bash
# 1. 改版本：cmd/bfserver/main.go 的默认 Version、README badge 与正文、tools/bfserver-install.sh 的 VERSION
#    ⚠️ 用 `-buildvcs=false` 构建：Go 默认会把 git revision 与「工作区是否被改过」写进二进制，
#    那样改一行文档哈希就变，BIN_SHA256 永远对不上。加了这个开关后产物只由源码与 flag 决定。
# 2. 构建两端产物（Linux 供一键脚本，Windows 供手动测试）
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.Version=0.4.0" -o bfserver ./cmd/bfserver
#    用固定 mtime/属主打包，tar.gz 才可复现（否则每次都不同，跟 Go 无关）
tar --sort=name --mtime='UTC 2026-01-01' --owner=0 --group=0 --numeric-owner \
    -czf bfserver-linux-amd64-v0.4.0.tar.gz bfserver
# 3. 算 tar.gz 的 sha256，写回 tools/bfserver-install.sh 的 BIN_SHA256
sha256sum bfserver-linux-amd64-v0.4.0.tar.gz
# 4. 提交、打 tag、推 main 与 tag，然后在 GitHub 建 Release 并上传两个资产：
#    bfserver-linux-amd64-v0.4.0.tar.gz、bfserver-install.sh
git tag -a v0.4.0 -m "v0.4.0"
git push origin main
git push origin v0.4.0
```
