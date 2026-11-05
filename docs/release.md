# 发布流程（打 tag 与上传资产）

改版本号时按顺序做，四步都要对上，否则一键脚本会 404：

```bash
# 1. 改版本：cmd/bfserver/main.go 的默认 Version、README badge 与正文、tools/bfserver-install.sh 的 VERSION
# 2. 构建两端产物（Linux 供一键脚本，Windows 供手动测试）
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=0.3.2" -o bfserver ./cmd/bfserver
tar czf bfserver-linux-amd64-v0.3.2.tar.gz bfserver
# 3. 算 tar.gz 的 sha256，写回 tools/bfserver-install.sh 的 BIN_SHA256
sha256sum bfserver-linux-amd64-v0.3.2.tar.gz
# 4. 提交、打 tag、推 main 与 tag，然后在 GitHub 建 Release 并上传两个资产：
#    bfserver-linux-amd64-v0.3.2.tar.gz、bfserver-install.sh
git tag -a v0.3.2 -m "v0.3.2"
git push origin main
git push origin v0.3.2
```
