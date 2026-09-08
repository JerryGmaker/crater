# 分页任务推送清单

## 当前状态

分支：`feature/pagination`

本地领先 `origin/feature/pagination` 的 4 个提交：

1. `0334737 test(pagination): automate EMIAS acceptance`
2. `5a6e199 test(frontend): guard EMIAS billing behavior`
3. `bb9b070 fix(test): make backend suite hermetic`
4. `8e5ff31 docs(pagination): record final acceptance blockers`

以上提交均包含：

```text
Signed-off-by: JeryGmaker <realgjt@163.com>
```

## 推送前检查

在网络恢复且 GitHub 登录状态正常后，在仓库根目录执行：

```powershell
git status --short --branch
git log origin/feature/pagination..HEAD --oneline
git log origin/feature/pagination..HEAD --format='%h %s%n%(trailers:key=Signed-off-by,valueonly)'
git diff --check origin/feature/pagination..HEAD
```

确认工作树干净、4 个提交均有 DCO 后执行：

```powershell
git push origin feature/pagination
```

推送后检查：

```powershell
git fetch origin feature/pagination
git status --short --branch
```

## 需要真实环境变量的验收

Token 只在本机环境变量中设置，不要提交或发送到聊天：

```powershell
$env:CRATER_USER_AUTH_TOKEN = "<普通用户 Token>"
$env:CRATER_ADMIN_AUTH_TOKEN = "<管理员 Token>"
node hack/check-emias-pagination.mjs
node hack/check-image-detail.mjs
```

两个脚本只发送 GET 请求；CI 只运行离线假服务测试，不会连接实验室环境。
