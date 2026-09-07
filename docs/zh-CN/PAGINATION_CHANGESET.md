# 分页改动整理清单

## 已提交的分页工作

当前 `feature/pagination` 分支已经包含分页架构、页面迁移、自动验收脚本、Swagger 静态检查和 CI 工作流等提交。最近的两个提交是：

- `de401f5 docs: document platform pagination architecture`
- `daa4ce8 test: automate pagination acceptance checks`

## 当前工作区的整理结果

当前工作区仍有未提交文件。根据 `git diff --name-only`，实际存在内容差异的文件主要是：

- `backend/internal/handler/aijob/list.go`
- `backend/internal/handler/aijob/list_test.go`
- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`

其中 EMIAS handler 和测试属于同一项“EMIAS 文档/行为测试收口”工作。Swagger 三个生成文件虽然包含 EMIAS 新路径，但本次生成同时带入了其他 API 的变更（例如 QueueQuota），因此不能直接作为纯分页提交；应在干净基线或完成差异拆分后重新生成。`backend/internal/service/operation_log.go` 只是本地启动时跳过迁移的辅助开关，不属于分页改动。

本轮另外新增了一处已验证的前端修正：

- `frontend/src/routes/admin/accounts/-components/account-table.tsx`：将账户名称输入从列筛选改为远程全局搜索，使请求使用公共 `search` 参数。真实页面已验证搜索 `test` 返回 2 条、无匹配搜索返回 0 条。
- `frontend/src/routes/admin/users/index.tsx`：将用户名输入从列筛选改为远程全局搜索，使请求使用公共 `search` 参数。真实页面已验证搜索 `guanjt` 返回 1 条、无匹配搜索返回 0 条。
- `frontend/src/components/file/data-detail.tsx`：将用户共享和账户共享页签从列筛选改为远程全局搜索，使共享成员查询使用公共 `search` 参数；真实页面已验证已有记录和无匹配空结果。
- `hack/check-pagination-static.mjs`：加入管理账户页面和 `/v1/admin/accounts/page` 的静态防回归检查。
- `hack/check-pagination-static.mjs`：加入管理用户页面和 `/v1/admin/users/page` 的静态防回归检查。
- `hack/check-pagination-static.mjs`：加入共享文件、数据集共享成员页面及三个数据集分页路由的静态防回归检查。
- `hack/check-pagination-static.mjs`：加入镜像用户/管理端分页路由和共用 `ImageListTable` 的静态防回归检查。
- `frontend/src/components/model/model-downloads-page.tsx`：把后端分页总数传给通用 `DataTablePagination`，修正底部总数只显示当前页行数的问题；真实页面已验证 62 条总数、页大小、搜索和空结果。
- `frontend/src/routes/admin/operation-logs/index.tsx`：将操作日志分页默认排序及表头排序 ID 从数据库字段名改为协议字段名 `createdAt`、`operationType`，修复页面初始请求 400 的问题；真实页面已验证分页、搜索、时间筛选、类型筛选和空结果。
- `hack/check-pagination-static.mjs`：加入定时任务记录分页路由和 `CronJobRecordsTable` 远程模式的静态防回归检查；真实页面已验证搜索、状态筛选、空结果和页大小。
- `hack/check-pagination-static.mjs`：加入模型下载分页路由和页面总数传递的静态防回归检查。
- `frontend/src/routes/portal/data/datasets/index.tsx` 与 `frontend/src/components/file/data-view.tsx`：将数据集主列表接入已有远程分页接口；模型首页继续保留本地组织聚合逻辑。真实 Network 验收已确认初始列表、翻页、页大小、搜索和空结果均正常。

本轮真实验收还确认：`/portal/data/blocks` 使用共享文件远程分页入口，但当前真实环境为空；数据集主列表已完成真实 Network 验收。

镜像管理页面本轮已完成真实只读验收：96 条数据、翻页无重复、搜索、空结果和页大小变化均正常。

镜像构建页面本轮已完成真实只读验收：管理端 162 条数据，已验证翻页无重复、页大小、搜索、空结果、创建时间排序和失败状态筛选；普通用户端当前权限范围为空，但状态筛选控件已确认可用。为修正原先状态筛选按钮错误置灰的问题，`frontend/src/components/image/registry/index.tsx` 已将状态过滤声明为 `remoteFacets: true`。页面刷新时出现的按名称详情预取提示与分页列表请求分离，暂不纳入分页提交。

集群资源页完成代码盘点和协议对齐：后端 `/v1/resources/page` 已使用公共 `search`、重复 `type` 筛选、排序白名单和 `id` 稳定排序；前端名称输入已从 `name` 列过滤改为全局远程搜索，资源类型筛选声明为 `remoteFacets: true`。本轮补充了搜索/类型先于分页、稳定排序、空结果和越界页码测试，真实页面验收待后续在管理员会话中完成。

## 明确不纳入本次分页整理

工作区还显示一批后端配置、用户、认证、计费、模型下载配额、预队列监听器、重协调器等文件变化。这些文件不应在本次分页整理中被批量加入、重排或回滚；它们保留给原有工作流的负责人单独确认。

整理原则：

1. 不使用 `git add .`。
2. 分页 changeset 只允许加入能由对应测试、Swagger 或页面验收解释的文件。
3. 生成的 Swagger 文件必须与注释源同步生成，不能手工只改其中一个。
4. 在确认 EMIAS 配置和测试边界前，不提交实验室账号、kubeconfig、debug config 或任何密钥。

因此本轮只完成“识别和隔离”，没有擅自提交上述混合工作区改动，也没有回滚它们。

## 下一次提交前检查

```shell
git diff --name-only
git diff -- backend/internal/handler/aijob/list.go backend/internal/handler/aijob/list_test.go
node hack/check-pagination-static.mjs
node --test hack/check-pagination.test.mjs
```

审阅通过后，再只对 EMIAS 相关文件执行显式 `git add`，不要影响工作区中的其他变动。
