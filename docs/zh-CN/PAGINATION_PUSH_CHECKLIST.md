# 分页任务推送清单

## 当前状态

分支：`feature/pagination`

本地领先 `origin/feature/pagination` 的 7 个提交：

1. `126b0b3 docs(pagination): record authenticated acceptance evidence`
2. `249cd79 docs(pagination): prepare delivery and snapshot design`
3. `f2d2290 fix(image): avoid duplicate kaniko detail lookup`
4. `58edaf2 test(dco): add repeatable commit audit`
5. `02069ab docs(pagination): review mixed history and compatibility`
6. `8bd8010 test(pagination): harden CI and image detail regression checks`
7. 本次同步状态记录提交（即当前文档所在提交）

上述提交均包含：

```text
Signed-off-by: JeryGmaker <realgjt@163.com>
```

### 分支历史 DCO 审计

以本地 `main` 为基线审计 `main..feature/pagination` 共 55 个提交：52 个包含上述 DCO，以下 3 个历史提交缺少签署：

- `62da045 feat(cli): 补齐 job ls 服务端筛选参数 / expose server-side job filters (#483)`
- `1b67f29 fix(node): show pod start time in node workloads (#505)`
- `56ce3a4 feat(frontend): redesign cron job policy cards (#503)`

这 3 个提交早于本次分页整理且不属于分页改动。当前不重写历史、不压缩提交；若目标仓库要求每个提交均有 DCO，应在推送前由提交作者补签，或由负责人明确授权后再制定可审计的历史重写方案。

### Upstream 同步状态（2026-09-09）

- `upstream/main` 已更新到 `62da045`。
- `upstream/main` 已是 `feature/pagination` 的祖先，当前无需 rebase，也没有冲突。
- 当前分支相对 `upstream/main` 有 52 个提交，相对 fork 的 `origin/feature/pagination` 有 7 个待推送提交。
- HTTPS fetch 曾因 GitHub 443 连接失败；已通过 SSH 只读探测和 fetch 成功完成同步。未修改上游代码，也未推送任何远程分支。

## 推送前检查

在网络恢复且 GitHub 登录状态正常后，在仓库根目录执行：

```powershell
git status --short --branch
git log origin/feature/pagination..HEAD --oneline
git log origin/feature/pagination..HEAD --format='%h %s%n%(trailers:key=Signed-off-by,valueonly)'
git diff --check origin/feature/pagination..HEAD
node hack/check-dco.mjs origin/feature/pagination HEAD
```

确认工作树干净、待推送提交带 DCO，并按上面的历史审计结果处理缺失签署后再执行：

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

## 本地 PR 说明草稿（暂不推送）

### 变更摘要

- 将数据库主列表统一接入远程分页、搜索、排序和总数协议，覆盖 JobTemplate、账户/用户、数据集、镜像、镜像构建、模型下载、操作日志、定时任务记录、集群资源和 EMIAS。
- 修正 EMIAS 不支持的 billing 请求，补齐测试初始化、operation-log 查询、reconciler 数据库注入和队列测试隔离。
- 新增 EMIAS 分页与镜像详情的只读验收脚本、离线测试、静态检查和 CI 接入。
- 已在真实登录会话和实验室数据上完成 EMIAS、镜像构建列表及稳定 ID 详情验收。

### 验证结果

- 后端 `go test ./...`、`go vet ./...`：通过。
- 前端分页测试 9/9、TypeScript、ESLint、生产构建：通过（仅已有构建警告）。
- 离线验收脚本 3/3、Swagger/前端静态检查：通过。
- EMIAS 真实数据：个人/账户/管理员分别 6/8/8 条，权限包含关系通过。
- 镜像详情真实数据：管理员列表 162 条，稳定 ID 详情字段一致；普通用户当前无镜像构建记录。

### 当前交付边界

- 本地分支已准备好，但按当前决定暂不执行 `git push`。
- Kubernetes 节点/Pod 快照和文件目录不直接套用数据库页码协议，专项设计见 `PAGINATION_NON_DATABASE_DESIGN.md`。
- 分支差异审查和混合提交说明见 `PAGINATION_DIFF_REVIEW.md`；DCO 可重复审计脚本为 `hack/check-dco.mjs`。
