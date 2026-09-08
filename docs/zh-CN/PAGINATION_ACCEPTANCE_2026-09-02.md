# 分页真实数据验收记录

## 1. 验收目的

记录分页协议在本地前后端服务和实验室真实只读数据上的验收结果，并把“代码/编译检查”和“真实页面 Network 验收”分开。

## 2. 验收环境

- 日期：2026-09-02
- 数据集主列表补验：2026-09-07
- 前端：`http://localhost:5173`
- 后端：`http://localhost:8088`
- 访问方式：Easy Connect 已连接；本地服务通过临时开发配置访问实验室依赖
- 操作范围：仅执行登录、页面浏览和 GET 查询；没有创建、修改或删除实验室数据
- 账号范围：普通用户页面使用普通用户权限；管理页面只在已有管理员权限的会话中检查

## 3. 统一验收标准

每个远程列表应满足：

```text
页面 query
-> GET 携带 page/page_size/search/sort 及业务筛选
-> 后端按权限、搜索、筛选、排序、稳定排序、分页处理
-> 返回 items/total/page/page_size
-> 前端只展示当前页，不再次本地分页、搜索或排序
```

## 4. 真实页面结果

| 页面                   | 请求结果   | 关键证据                                                                                                                                                                                              | 结论                                                |
| ---------------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| 作业模板 JobTemplate   | `200 OK`   | `GET /api/v1/jobtemplate?page=1&page_size=10&owner=all&sort=-createdAt`；响应包含 `items/total/page/page_size`，真实数据 `total=13`、第一页 10 条；第二页 3 条且与第一页无重复                        | 通过                                                |
| 作业模板搜索           | `200 OK`   | `search=测试` 返回 `total=3`；无匹配搜索返回 `total=0`、`items=[]`                                                                                                                                    | 通过                                                |
| 作业模板页大小         | `200 OK`   | `page_size=20` 返回 `total=13`、`items` 13 条                                                                                                                                                         | 通过                                                |
| 普通作业 VCJob         | `200 OK`   | `/api/v1/vcjobs?page=1&page_size=10&sort=-createdAt...` 及 facets 请求成功；当前账号可见 `total=0`，facets 为空                                                                                       | 通过，空数据是当前权限范围的真实结果                |
| 审批工单 ApprovalOrder | `200 OK`   | `/api/v1/approvalorder/page?page=1&page_size=10&sort=-createdAt` 成功；当前账号可见 `total=0`、`items=[]`                                                                                             | 通过，空数据是当前权限范围的真实结果                |
| GPU Analysis           | `200 OK`   | 使用白名单字段 `sort=-CreatedAt` 查询成功，响应为标准分页结构；使用错误大小写 `sort=-createdAt` 返回 `400`，说明排序白名单生效                                                                        | 通过，当前返回空数据                                |
| 管理数据集             | `200 OK`   | `/admin/data` 真实页面显示 `共 60 条`；第一页和第二页各 10 条且名称无重复；搜索 `Qwen` 后显示 `共 20 条`；页大小可切换为 20                                                                           | 通过                                                |
| 用户管理               | `200 OK`   | `/admin/users` 真实页面显示 `共 46 条`；搜索 `guanjt` 返回 1 条；无匹配搜索返回 `total=0`                                                                                                             | 通过                                                |
| 数据集主列表           | `200 OK`   | `/portal/data/datasets` 使用 `/api/v1/dataset/mydataset/page`；真实数据 `total=12`，第一页 10 条、第二页 2 条且无重复；页大小 20 返回 12 条；搜索 `Maynor` 返回 1 条；无匹配搜索返回 0 条并显示空状态 | 通过                                                |
| 共享文件               | 页面空结果 | `/portal/data/blocks` 使用 `apiGetDatasetPaged` 远程模式；当前小集群无共享文件，页面显示 `暂无数据`                                                                                                   | 空结果页面已确认；待有数据环境补充跨页 Network 证据 |
| 数据集共享成员         | `200 OK`   | 数据集详情用户共享和账户共享页签均显示 `共 1 条`；搜索无匹配后均变为 `暂无数据`、`total=0`                                                                                                            | 通过，已覆盖已有数据和空结果                        |
| 镜像                   | `200 OK`   | `/admin/env/images` 显示 `共 96 条`；第一页和第二页内容不同；搜索 `vllm` 返回 `共 10 条`；无匹配返回 `共 0 条`；页大小 20 后显示 20 行                                                                | 通过                                                |
| 模型下载               | `200 OK`   | `/portal/data/models/downloads` 显示 `共 62 条`；翻页内容变化；页大小 20 显示 20 行；搜索 `Maynor` 返回 1 条，搜索 `vllm` 返回 0 条                                                                   | 通过；同时修正分页底部曾误显示当前页行数的问题      |
| 操作日志               | `200 OK`   | `/admin/operation-logs` 显示 `共 89 条`；搜索 `DeleteJob` 返回 24 条；无匹配返回 0 条；近 7 天返回 1 条；类型筛选“取消独占”返回 1 条                                                                  | 通过；同时修正前端排序字段与后端白名单不一致的问题  |
| 定时任务记录           | `200 OK`   | `/admin/cronjobs` 显示 `共 1 条`；搜索 `clean-waiting-custom` 返回 1 条；无匹配返回 0 条；状态筛选“失败”返回 0 条；清除筛选后恢复 1 条；页大小 20 可用                                                | 通过；当前真实数据量不足以验证跨页不重复            |
| 账户成员               | `200 OK`   | `/admin/accounts/1` 真实数据 `total=46`；第 2 页与第 1 页无重复；页大小 20 返回 20 条；搜索 `guanjt` 返回 1 条，无匹配搜索返回 0 条                                                                   | 通过；前端搜索参数已统一为公共参数 `search`         |
| EMIAS                  | `200 OK`   | 2026-09-08 在本地调试配置启用 EMIAS plugin 后，个人、账户和管理范围分别返回 6、8、8 条真实 AIJob；已验证跨页无重复、总数稳定、页大小、越界空页、升/降序、真实搜索、状态筛选、facets 和权限范围        | 通过                                                |

| 镜像构建 | `200 OK` | 用户端 `/portal/env/registry` 在当前权限范围返回 `total=0`，状态筛选控件可用；管理端 `/admin/env/registry` 返回 `total=162`，第 1、2 页各 10 条且无重复，页大小 20、搜索 `gnn` 返回 4 条、无匹配搜索返回 0 条、创建时间升序生效 | 通过；状态筛选修正后重新验收 |
| 集群资源 | `200 OK` | `/admin/cluster/resources` 显示 `共 33 条`；第 1、2 页各 10 条且无重复；页大小 20；搜索 `cpu` 返回 3 条；无匹配搜索返回 0 条；类型筛选 `vGPU` 返回 3 条 | 通过；前端搜索已统一为 `search` |

### 4.1 JobTemplate 代表性 Network 证据

已核对以下请求：

```text
GET /api/v1/jobtemplate?page=1&page_size=10&owner=all&sort=-createdAt
GET /api/v1/jobtemplate?page=2&page_size=10&owner=all&sort=-createdAt
GET /api/v1/jobtemplate?page=1&page_size=20&owner=all&sort=-createdAt
GET /api/v1/jobtemplate?page=1&page_size=20&owner=all&sort=-createdAt&search=测试
```

翻页、页大小、搜索都会重新请求后端；搜索条件没有只作用于当前页，且响应总数随搜索结果变化。

### 4.2 管理数据集代表性验收

已核对管理数据页面：

```text
初始列表：共 60 条，10 条/页，共 6 页
第二页：与第一页的名称集合无重复
搜索 Qwen：共 20 条
页大小：可切换为 20 条/页
```

该页面使用了表格虚拟渲染，改变页大小后屏幕上同时可见的行数不一定等于后端返回条数；验收以分页控件、总数和查询结果为准。

### 4.3 用户管理代表性验收

已核对用户管理页面：

```text
初始列表：共 46 条，10 条/页，共 5 页
搜索 guanjt：共 1 条，结果为当前用户“关金侗”
无匹配搜索：共 0 条，页面显示空结果
```

### 4.4 共享文件与数据集共享成员

共享文件页的代码入口已经传入 `apiGetDatasetPaged` 和 `sourceType="sharefile"`，因此使用 `DataView` 的远程模式；本次真实环境没有共享文件，验收结果是空状态，不能据此证明跨页数据行为。

数据集详情的用户共享和账户共享页签均使用分页接口。当前选中的数据集各有 1 条真实记录；输入无匹配关键词后，列表变为空结果，说明搜索条件已经进入远程查询流程。

数据集主列表已经切换为 `apiGetDatasetPaged` 远程模式。本次真实 Network 验收确认页面 query 会发送 `page/page_size/search/sort` 及业务筛选，后端返回 `items/total/page/page_size`，前端显示当前页结果。

### 4.5 镜像列表代表性验收

已核对管理端镜像列表：

```text
初始列表：共 96 条，10 条/页，共 10 页
翻到第二页：记录内容与第一页不同
搜索 vllm：共 10 条
无匹配搜索：共 0 条，页面显示空结果
页大小切换为 20：页面显示 20 行，总数仍为 96 条
```

镜像列表通过 `ImageListTable` 的远程表格模式加载数据，用户端和管理端共用这套表格实现，具体请求函数由页面注入。

### 4.6 模型下载列表代表性验收

已核对模型下载列表：

```text
初始列表：共 62 条，10 条/页，共 7 页
翻页：第二页记录内容与第一页不同
页大小切换为 20：页面显示 20 行，总数仍为 62 条
搜索 Maynor：共 1 条
搜索 vllm：共 0 条，页面显示空结果
```

该页面原本已经根据后端 `total` 计算页数，但没有把 `total` 传给通用 `DataTablePagination`，导致底部总数显示当前页行数。现已补传 `totalItems={total}`，页数和总数显示一致。

### 4.7 操作日志列表代表性验收

已核对管理端操作日志列表：

```text
初始列表：共 89 条，10 条/页，共 9 页
翻页：第二页记录内容与第一页不同
搜索 DeleteJob：共 24 条
无匹配搜索：共 0 条，页面显示空结果
时间筛选近 7 天：共 1 条
操作类型筛选“取消独占”：共 1 条
```

验收时发现页面默认排序使用了数据库字段名 `created_at`，而分页接口协议要求 `createdAt`。现已将默认排序以及“操作时间”“操作类型”表头的排序 ID 改为协议字段，数据展示仍使用后端返回的 `created_at` 和 `operation_type`。

### 4.8 定时任务记录代表性验收

已核对管理端定时任务记录：

```text
初始列表：共 1 条，10 条/页，共 1 页
搜索 clean-waiting-custom：共 1 条
无匹配搜索：共 0 条，页面显示空结果
状态筛选“失败”：共 0 条
清除状态筛选：恢复为共 1 条
页大小切换为 20：查询仍正常，总数为 1 条
```

当前集群只有 1 条定时任务记录，因此本次只能验证分页协议、筛选和空结果，不能用真实数据验证跨页去重。

### 4.9 数据集主列表代表性验收

已在 VPN、前后端本地服务和普通用户登录会话下完成只读验收：

```text
初始列表：GET /api/v1/dataset/mydataset/page?page=1&page_size=10&owner=all&sort=-createdAt&type=dataset
初始响应：200 OK，total=12，page=1，page_size=10，items=10
第二页：page=2&page_size=10，200 OK，items=2；与第一页的 ID 集合无重复
页大小：page_size=20，200 OK，total=12，items=12
搜索 Maynor：search=Maynor，200 OK，total=1，结果为 Maynor996/upload2
空结果：search=__no_dataset_match__，200 OK，total=0，items=[]，页面显示“暂无数据”
```

所有请求都保留了 `owner=all`、`sort=-createdAt` 和 `type=dataset`，说明页码、页大小和搜索变化是通过远程 query 重新请求后端完成的，而不是前端对全量数组再次本地切分。

### 4.10 账户成员代表性验收

已在管理端公共账户（账户 ID `1`）完成只读验收：

```text
初始列表：GET /api/v1/admin/accounts/userIn/1/page?page=1&page_size=10&sort=name
初始响应：200 OK，total=46，page=1，page_size=10，items=10
第二页：page=2&page_size=20，200 OK，items=20；与第一页用户名集合无重复
页大小：page_size=20，200 OK，total=46，items=20
搜索 guanjt：使用公共搜索参数 search，200 OK，total=1，页面显示关金侗
空搜索：search=__account_member_no_match__，200 OK，total=0，页面显示“暂无数据”
```

账户成员表格已从 `name` 列过滤改为 DataList/DataTable 公共远程搜索。页面只持有一份 query，输入搜索词后由远程模式触发新请求，后端读取统一的 `search` 参数；搜索、空结果、页大小和跨页行为均已完成真实数据验收。

### 4.11 镜像构建代表性验收

已在管理员和普通用户页面完成只读验收：

```text
管理员初始列表：共 162 条，10 条/页
管理员翻页：第 1、2 页各 10 条，记录无重复
管理员页大小：切换为 20 条/页后仍显示共 162 条
管理员搜索 gnn：共 4 条，结果均匹配镜像名称
管理员无匹配搜索：共 0 条，页面显示暂无数据
管理员排序：创建时间升序后显示 1 年前记录，说明排序请求已生效
管理员状态筛选：选择“失败”后共 65 条，页面记录状态均为“失败”
普通用户页面：状态筛选控件可用；当前权限范围共 0 条，页面显示暂无数据
```

验收过程中发现状态筛选按钮原先因未声明远程 facet 而被错误置灰。现已为镜像构建页的状态筛选配置 `remoteFacets: true`，让固定的状态选项直接触发后端筛选，而不是依赖当前页数据生成本地 facet。该修正不改变数据，只改变查询条件的提交方式。

页面刷新期间曾出现一次 `fetch kaniko by name failed, err record not found` 提示。该提示来自列表外的镜像详情预取/按名称查询，与分页列表请求分离；本次分页验收未将它记为分页失败，后续可单独排查详情接口的历史记录兼容问题。

### 4.12 集群资源代表性验收

已在管理员会话下完成只读验收：

```text
初始列表：共 33 条，10 条/页，共 4 页
翻页：第 1、2 页各 10 条，资源记录无重复
页大小：切换为 20 条/页后仍显示共 33 条
搜索 cpu：共 3 条，结果为 cpu、batch-cpu、mid-cpu
无匹配搜索：共 0 条，页面显示暂无数据
类型筛选 vGPU：共 3 条，页面每条记录类型均为 vGPU
```

本次验收确认资源页的名称输入已经通过公共 `search` 参数触发后端搜索，类型筛选通过重复 `type` 参数触发后端筛选；页面没有对当前页数据再次执行本地搜索或分页。

### 4.13 镜像详情历史记录兼容修正

分页列表之外，单独处理了刷新镜像详情时偶发的 `record not found`：

- 列表进入详情时携带稳定的镜像构建记录 `id`，用户端和管理端分别按 ID 查询；
- 原 `/v1/images/getbyname` 保留用于历史链接兼容，同时允许兼容传入 `id`；
- 详情查询找不到记录时返回明确的 Not Found 错误，前端显示“页面未找到”，不再把数据库错误直接作为通用请求失败弹窗；
- 描述和 Dockerfile 为空时按空字符串返回，避免详情响应因空指针失败；
- 新增 `/v1/images/getbyid` 与 `/v1/admin/images/getbyid`，并已重新生成 Swagger。

该修正尚未使用实验室写操作；详情接口本身的真实数据验收仍应在 VPN 和有效登录会话下补做。分页列表的真实验收与此详情问题分开记录。

## 5. EMIAS 真实环境验收

2026-09-08 已解除“路由未注册”阻断。本地调试配置启用
`schedulerPlugins.aijob.enable` 并重启后端，启动日志确认
`Scheduler Plugins: EMIAS(profiling: false, timeout: 120s)`，AIJob CRD 和集群中的真实 AIJob 均可访问。

在同一个已登录会话中，对以下三组只读接口执行了真实数据验收：

1. 个人范围：`/api/v1/aijobs/page` 和 `/api/v1/aijobs/page/facets`，`total=6`；
2. 账户范围：`/api/v1/aijobs/all/page` 和 `/api/v1/aijobs/all/page/facets`，`total=8`；
3. 管理范围：`/api/v1/admin/aijobs/page` 和 `/api/v1/admin/aijobs/page/facets`，`total=8`。

三组接口均验证了第 1/2 页、`page_size=1/2`、总数在页码和页大小变化时保持稳定、
跨页不重复、创建时间升/降序、越界页返回空数组、facets 结构与键集。
另外使用真实作业名验证搜索返回 4 条，使用真实状态验证筛选结果和 facet 计数均为 4；
未带凭据的同一路由返回 `401`。权限范围满足“个人 ⊆ 账户 ⊆ 管理”。

前端将本地调度器切换为 `colocate` 后，`/portal/jobs/custom` 成功显示 5 条 EMIAS 作业、
真实总数和状态映射。验收过程中还发现该页误请求仅为 VCJob 注册的
`/aijobs/billing` 路由；已在 EMIAS 列表停用该查询和计费列，重启前端后页面无 `404` 通知。

仓库新增只读脚本 `hack/check-emias-pagination.mjs`，从运行时环境变量读取普通用户和
管理员 Token，不接受命令行 Token，也不输出凭据。脚本覆盖个人、账户、管理三种权限范围，
自动验证跨页、总数、页大小、排序、越界页、真实搜索、状态筛选、facets 和权限包含关系。
对应离线测试使用本地假服务运行，不连接实验室环境。

本次只调整了仓库外的本地调试配置，没有修改或写入集群 AIJob。

## 6. 自动检查结果

| 检查                           | 命令                                                                                                                                                 | 结果                                                                                   |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | ------------------------ | ---- |
| 前端分页状态与 EMIAS 计费守卫  | `pnpm test:pagination`                                                                                                                               | 通过，9/9；覆盖搜索/筛选/排序/页大小回到第一页，以及 EMIAS 不请求未注册的 billing 路由 |
| TypeScript                     | `pnpm exec tsc --noEmit`                                                                                                                             | 通过                                                                                   |
| 前端构建                       | `pnpm build`                                                                                                                                         | 通过，只有已有构建警告                                                                 |
| EMIAS handler 测试             | `CRATER_DEBUG_CONFIG_PATH=<local-debug-config> CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler/aijob -count=1`                      | 通过                                                                                   |
| 分页离线验收脚本               | `node --test hack/check-pagination.test.mjs hack/check-emias-pagination.test.mjs`                                                                    | 通过，2/2                                                                              |
| Swagger/前端静态检查           | `node hack/check-pagination-static.mjs`                                                                                                              | 通过，检查到 35 个 Swagger 分页路径和 17 个代表页面守卫                                |
| 镜像构建 handler 测试          | `CRATER_DEBUG_CONFIG_PATH=/tmp/crater-debug-config.yaml CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler/image -count=1`             | 通过                                                                                   |
| 集群资源 handler 测试          | `CRATER_DEBUG_CONFIG_PATH=/tmp/crater-debug-config.yaml CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler -run 'Test(ListResourcePage | BindResource                                                                           | ResourcePage)' -count=1` | 通过 |
| 图片详情参数/列表 handler 测试 | `CRATER_DEBUG_CONFIG_PATH=/tmp/crater-debug-config.yaml CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler/image -count=1`             | 通过                                                                                   |
| 后端全量测试                   | `GIN_MODE=debug CRATER_DEBUG_CONFIG_PATH=<absolute-example-config> CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./... -count=1`                     | 通过                                                                                   |

为使干净 checkout 的全量测试可重复运行，本轮同时修正了以下测试基础问题：测试模式按 debug
配置路径初始化；operation-log 测试正确编码 URL 搜索参数；reconciler 使用注入数据库而不是
全局数据库；优先队列恢复标准堆语义；注册事务中的计费初始化保持同一个生成查询事务，避免
模型表上下文污染。`CRATER_DEBUG_CONFIG_PATH` 必须使用绝对路径，因为 Go 会在各包目录中运行测试。

自动检查证明代码结构、协议形状和离线逻辑满足预期；真实 Network 验收则证明当前环境中页面确实发出了分页请求，两者不互相替代。

## 7. 后续动作

- 在 VPN、后端和有效登录会话可用时运行 `hack/check-emias-pagination.mjs`，持续回归权限范围和状态 facets。
- 按覆盖矩阵逐项确认仍使用本地模式的列表是否真的需要分页。
- 对明确需要分页的剩余数据库主列表先补协议/权限/排序设计，再开发；不为 Kubernetes 快照和文件目录强行套用数据库分页。

## 8. 2026-09-09 收尾检查

- 本机 `127.0.0.1:8088` 和 `127.0.0.1:5173` 的 `/api/auth/mode` 均返回 `200`，前后端服务正常。
- 匿名访问 EMIAS 分页和镜像详情接口均返回 `401`，说明路由和鉴权中间件已生效；本次会话没有可安全用于真实验收的登录 Token，因此没有把匿名结果冒充真实数据通过。
- EMIAS 真实验收命令已准备好：
  `CRATER_USER_AUTH_TOKEN=... CRATER_ADMIN_AUTH_TOKEN=... node hack/check-emias-pagination.mjs`。
- 镜像详情真实只读验收应在登录后访问 `/api/v1/images/getbyid?id=<真实记录 ID>` 和管理员对应路由；代码测试与路由均已通过，当前剩余的是认证数据条件。
- 推送 `feature/pagination` 到 fork 的尝试因当前主机连接 GitHub `443` 超时而未完成；本地分支仍保留 3 个带 DCO 提交，未丢失任何代码。
