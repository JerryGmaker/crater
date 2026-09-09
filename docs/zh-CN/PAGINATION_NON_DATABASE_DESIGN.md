# Kubernetes 快照与文件目录分页专项设计

本文记录不适合直接复用数据库 `page/page_size/total` 协议的三个页面类别：Kubernetes 节点快照、作业 Pod 快照和文件目录。当前只完成协议边界设计，不改变现有接口。

## 1. 共同原则

- 不把实时快照或目录扫描伪装成数据库总数分页。
- 使用不透明游标，客户端不得解析或自行拼接游标。
- 每次响应返回 `items`、`next_cursor` 和 `has_more`；`total` 可选且默认不提供。
- 首次请求创建短生命周期快照或目录读取上下文；后续请求携带同一游标，避免翻页期间集合漂移。
- 游标过期、资源版本变化或路径失效时返回明确错误，前端回到第一页并提示刷新。
- 只读列表不触发 Kubernetes、存储系统或数据库写操作。

## 2. 节点与 Pod 快照

### 建议请求

```text
GET /api/v1/resources/snapshot?limit=50&cursor=<opaque>&selector=<label-selector>&resource_version=<rv>
GET /api/v1/jobs/{job}/pods/snapshot?limit=50&cursor=<opaque>&resource_version=<rv>
```

### 建议响应

```json
{
  "items": [],
  "next_cursor": "<opaque>",
  "has_more": true,
  "snapshot_id": "<opaque>",
  "resource_version": "<rv>",
  "observed_at": "2026-09-09T10:00:00+08:00"
}
```

- 节点快照按 `metadata.name` 排序，Pod 快照按 `metadata.creationTimestamp` 与名称稳定排序。
- `resource_version` 记录 Kubernetes list/watch 的一致性边界；不承诺跨快照 `total` 稳定。
- 大规模节点列表优先使用 Kubernetes `limit/continue`，不要在服务端先拉全量再切片。
- 需要实时状态时，通过 watch 或短轮询刷新快照，而不是在同一页码上强行合并变化。

## 3. 文件目录

### 建议请求

```text
GET /api/v1/storage/list?path=<normalized-path>&limit=100&cursor=<opaque>&sort=name
```

### 建议响应

```json
{
  "items": [],
  "next_cursor": "<opaque>",
  "has_more": false,
  "path": "/datasets/example",
  "observed_at": "2026-09-09T10:00:00+08:00"
}
```

- `path` 必须经过规范化和权限校验，禁止通过游标或路径绕过根目录限制。
- 默认按名称加文件类型稳定排序；同名项使用后端唯一标识作为 tie-breaker。
- 不返回目录 `total`，因为对象存储/文件服务的全量计数可能昂贵且在读取期间变化。
- 目录变化时允许返回 `cursor_expired` 或 `snapshot_changed`，前端清除游标并重新读取当前目录。

## 4. 前端与验收要求

- 使用独立的 `CursorList`/`SnapshotList` 组件，不复用数据库 `DataList` 的页码控件。
- UI 显示“加载更多/刷新快照”，不显示误导性的“第 N 页/共 M 条”。
- 离线测试覆盖空结果、重复游标、过期游标、资源版本变化、路径越权和稳定排序。
- 真实验收需记录快照时间、资源版本/目录读取上下文、跨游标无重复和刷新后的可恢复性。
- 在节点/Pod 或目录数据规模、刷新频率和后端接口明确前，保持现有页面实现不变。
