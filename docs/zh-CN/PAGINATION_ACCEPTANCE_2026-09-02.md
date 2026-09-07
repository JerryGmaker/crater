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

| 页面 | 请求结果 | 关键证据 | 结论 |
| --- | --- | --- | --- |
| 作业模板 JobTemplate | `200 OK` | `GET /api/v1/jobtemplate?page=1&page_size=10&owner=all&sort=-createdAt`；响应包含 `items/total/page/page_size`，真实数据 `total=13`、第一页 10 条；第二页 3 条且与第一页无重复 | 通过 |
| 作业模板搜索 | `200 OK` | `search=测试` 返回 `total=3`；无匹配搜索返回 `total=0`、`items=[]` | 通过 |
| 作业模板页大小 | `200 OK` | `page_size=20` 返回 `total=13`、`items` 13 条 | 通过 |
| 普通作业 VCJob | `200 OK` | `/api/v1/vcjobs?page=1&page_size=10&sort=-createdAt...` 及 facets 请求成功；当前账号可见 `total=0`，facets 为空 | 通过，空数据是当前权限范围的真实结果 |
| 审批工单 ApprovalOrder | `200 OK` | `/api/v1/approvalorder/page?page=1&page_size=10&sort=-createdAt` 成功；当前账号可见 `total=0`、`items=[]` | 通过，空数据是当前权限范围的真实结果 |
| GPU Analysis | `200 OK` | 使用白名单字段 `sort=-CreatedAt` 查询成功，响应为标准分页结构；使用错误大小写 `sort=-createdAt` 返回 `400`，说明排序白名单生效 | 通过，当前返回空数据 |
| 管理数据集 | `200 OK` | `/admin/data` 真实页面显示 `共 60 条`；第一页和第二页各 10 条且名称无重复；搜索 `Qwen` 后显示 `共 20 条`；页大小可切换为 20 | 通过 |
| 用户管理 | `200 OK` | `/admin/users` 真实页面显示 `共 46 条`；搜索 `guanjt` 返回 1 条；无匹配搜索返回 `total=0` | 通过 |
| 数据集主列表 | `200 OK` | `/portal/data/datasets` 使用 `/api/v1/dataset/mydataset/page`；真实数据 `total=12`，第一页 10 条、第二页 2 条且无重复；页大小 20 返回 12 条；搜索 `Maynor` 返回 1 条；无匹配搜索返回 0 条并显示空状态 | 通过 |
| 共享文件 | 页面空结果 | `/portal/data/blocks` 使用 `apiGetDatasetPaged` 远程模式；当前小集群无共享文件，页面显示 `暂无数据` | 空结果页面已确认；待有数据环境补充跨页 Network 证据 |
| 数据集共享成员 | `200 OK` | 数据集详情用户共享和账户共享页签均显示 `共 1 条`；搜索无匹配后均变为 `暂无数据`、`total=0` | 通过，已覆盖已有数据和空结果 |
| 镜像 | `200 OK` | `/admin/env/images` 显示 `共 96 条`；第一页和第二页内容不同；搜索 `vllm` 返回 `共 10 条`；无匹配返回 `共 0 条`；页大小 20 后显示 20 行 | 通过 |
| 模型下载 | `200 OK` | `/portal/data/models/downloads` 显示 `共 62 条`；翻页内容变化；页大小 20 显示 20 行；搜索 `Maynor` 返回 1 条，搜索 `vllm` 返回 0 条 | 通过；同时修正分页底部曾误显示当前页行数的问题 |
| 操作日志 | `200 OK` | `/admin/operation-logs` 显示 `共 89 条`；搜索 `DeleteJob` 返回 24 条；无匹配返回 0 条；近 7 天返回 1 条；类型筛选“取消独占”返回 1 条 | 通过；同时修正前端排序字段与后端白名单不一致的问题 |
| 定时任务记录 | `200 OK` | `/admin/cronjobs` 显示 `共 1 条`；搜索 `clean-waiting-custom` 返回 1 条；无匹配返回 0 条；状态筛选“失败”返回 0 条；清除筛选后恢复 1 条；页大小 20 可用 | 通过；当前真实数据量不足以验证跨页不重复 |
| 账户成员 | `200 OK` | `/admin/accounts/1` 真实数据 `total=46`；第 2 页与第 1 页无重复；页大小 20 返回 20 条；搜索 `guanjt` 返回 1 条，无匹配搜索返回 0 条 | 通过；前端搜索参数已统一为公共参数 `search` |
| EMIAS | `404` | `/api/v1/aijobs/page` 在当前小集群未注册；服务配置中 EMIAS scheduler plugin 未启用，代码会按配置跳过该路由 | 阻断，不能记为失败 |

| 镜像构建 | `200 OK` | 用户端 `/portal/env/registry` 在当前权限范围返回 `total=0`，状态筛选控件可用；管理端 `/admin/env/registry` 返回 `total=162`，第 1、2 页各 10 条且无重复，页大小 20、搜索 `gnn` 返回 4 条、无匹配搜索返回 0 条、创建时间升序生效 | 通过；状态筛选修正后重新验收 |

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

## 5. 阻断项与边界

EMIAS 的分页路由已进入代码和 Swagger，但当前服务配置未启用 EMIAS，因此真实页面无法用当前小集群完成验收。需要在 EMIAS enabled 的环境重新执行：

1. 普通用户 EMIAS 列表和 facets；
2. 管理员 EMIAS 列表和 facets；
3. 状态映射、权限范围和跨页稳定排序。

这项阻断不影响已完成的离线 handler 测试，也不应被描述为“EMIAS 真实数据验收通过”。

## 6. 自动检查结果

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| DataList / RemoteDataTable | `pnpm test:data-list` | 通过，6/6 |
| TypeScript | `pnpm exec tsc --noEmit` | 通过 |
| 前端构建 | `pnpm build` | 通过，只有已有构建警告 |
| EMIAS handler 测试 | `CRATER_DEBUG_CONFIG_PATH=/tmp/crater-debug-config.yaml CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler/aijob` | 通过 |
| 分页离线验收脚本 | `node --test hack/check-pagination.test.mjs` | 通过 |
| Swagger/前端静态检查 | `node hack/check-pagination-static.mjs` | 通过，检查到 34 个 Swagger 分页路径和 16 个代表页面守卫 |
| 镜像构建 handler 测试 | `CRATER_DEBUG_CONFIG_PATH=/tmp/crater-debug-config.yaml CRATER_SKIP_OPERATION_LOG_MIGRATION=1 go test ./internal/handler/image -count=1` | 通过 |

自动检查证明代码结构、协议形状和离线逻辑满足预期；真实 Network 验收则证明当前环境中页面确实发出了分页请求，两者不互相替代。

## 7. 后续动作

- 在 EMIAS enabled 环境补做真实数据验收。
- 按覆盖矩阵逐项确认仍使用本地模式的列表是否真的需要分页。
- 对明确需要分页的剩余数据库主列表先补协议/权限/排序设计，再开发；不为 Kubernetes 快照和文件目录强行套用数据库分页。
