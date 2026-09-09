# 分页改动整理清单

## 已提交的分页工作

当前 `feature/pagination` 分支已经包含分页架构、页面迁移、自动验收脚本、Swagger 静态检查和 CI 工作流等提交。最近的相关提交是：

- `c9e00ba ci: use absolute config path for pagination tests`
- `8bdbe13 test: cover image build pagination acceptance`
- `ddf6a4f docs: close pagination coverage matrix`
- `2844090 test: extend pagination static guards`
- `4899634 fix: align resource pagination search`

本轮新增的镜像详情兼容修正已独立提交：`2c2af55 fix: resolve image build detail by stable id`；验收边界记录已提交：`73f61d2 docs: record image detail acceptance boundary`。提交涉及的文件范围为：

- `backend/internal/handler/image/buildmgr.go`
- `backend/internal/handler/image/interface.go`
- `backend/internal/handler/image/types.go`
- `backend/internal/handler/image/buildmgr_test.go`
- `frontend/src/services/api/imagepack.ts`
- `frontend/src/services/api/admin/imagepack.ts`
- `frontend/src/services/query/image.ts`
- `frontend/src/components/layout/detail-page.tsx`
- `frontend/src/components/image/registry/index.tsx`
- `frontend/src/components/image/registry/registry-detail.tsx`
- `frontend/src/routes/portal/env/registry/$name.tsx`
- `frontend/src/routes/admin/env/registry/$name.tsx`
- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`

这组改动解决的是镜像详情按名称查询的独立兼容问题，不改变分页协议，也不应与其他未完成的业务改动混入同一提交。

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

镜像构建页面本轮已完成真实只读验收：管理端 162 条数据，已验证翻页无重复、页大小、搜索、空结果、创建时间排序、失败状态筛选，以及稳定 ID 详情（真实 ID `10768`）字段一致；普通用户端当前权限范围为空，但状态筛选控件已确认可用。为修正原先状态筛选按钮错误置灰的问题，`frontend/src/components/image/registry/index.tsx` 已将状态过滤声明为 `remoteFacets: true`。旧按名称详情接口保留用于历史深链接，稳定 ID 详情不再重复查询已加载的数据库记录。

集群资源页完成代码盘点和协议对齐：后端 `/v1/resources/page` 已使用公共 `search`、重复 `type` 筛选、排序白名单和 `id` 稳定排序；前端名称输入已从 `name` 列过滤改为全局远程搜索，资源类型筛选声明为 `remoteFacets: true`。本轮补充了搜索/类型先于分页、稳定排序、空结果和越界页码测试；真实页面已验证翻页无重复、页大小、搜索、空结果和 `vGPU` 类型筛选。

自动验收脚本现已覆盖 JobTemplate、VCJob、ApprovalOrder、GPU Analysis、用户/管理端镜像构建和 EMIAS 共 7 类接口，并检查分页响应结构、搜索/筛选条件、页大小、跨页重复和越界页。GitHub Actions 同步执行静态 Swagger/前端守卫、离线验收、镜像构建 handler 测试、DataList 测试、TypeScript 检查和前端构建；镜像构建测试使用仓库绝对配置路径，避免 Go 测试包工作目录导致的假失败。

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
