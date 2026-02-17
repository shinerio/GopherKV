# GopherKV Tasks

## Phase 1: MVP（最小可测试闭环）

- [x] T1 项目骨架、配置加载与日志（`cmd/internal/pkg/configs` 目录结构、默认配置、YAML 文件加载、`slog` 初始化）
- [x] T2 协议与错误码（统一响应结构 `{"code", "data", "msg"}` + 错误码常量映射）
- [x] T3 分片并发存储引擎（`RWMutex` + shard map，实现 `Set/Get/Delete/Exists/Keys/MemUsage`）
- [x] T4 数据约束与内存上限（key/value 长度校验 + maxmemory 拒写）
- [x] T5 TTL（惰性删除 + 小顶堆后台过期清理 + `TTL()` 查询）
- [x] T6 Core Service 层（`internal/core/service.go` Coordinator，编排 Storage + TTL，对上层暴露业务方法）
- [x] T7 HTTP Server（`PUT/GET/DELETE /v1/key` + `GET /v1/health`，调用 Service 层）
- [x] T8 SDK Client（`pkg/client`，薄封装 `http.Client` + JSON 序列化，实现 `Set/Get/Del/Exists/TTL`）
- [x] T9 CLI（`cmd/kvcli` REPL，基于 SDK，支持 `set/get/del/exists/ttl/help/exit`）
- [x] T10 优[tasks.md](tasks.md)雅停机（监听 `SIGINT/SIGTERM` → `http.Server.Shutdown` → 停止 TTL goroutine → 关闭存储引擎，超时保护）
- [ ] T11 MVP 测试与验收（`go test ./...` 通过，含核心存储单测 + `go test -race`）

## Phase 2: 高级能力补齐

- [ ] T12 AOF 持久化（append/replay/损坏截断恢复）
- [ ] T13 AOF Rewrite（阈值检测 + rewrite 流程 + 增量缓冲）→ 依赖 T12
- [ ] T14 RDB 快照（手动触发 `POST /v1/snapshot` + 自动规则触发 + 文件落盘）→ 依赖 T12
- [ ] T15 双持久化协同（AOF 优先恢复、停机 flush AOF + RDB snapshot）→ 依赖 T13, T14
- [ ] T16 监控统计（keys/memory/hits/misses/requests/uptime + `GET /v1/stats` + CLI `stats`）
- [ ] T17 安全扩展点（HTTP middleware hook 预留）
- [ ] T18 配置与文档收敛（`configs/config.yaml` 完善 + `README.md` 更新）
- [ ] T19 全量回归与性能验收（全功能回归 + 压测吞吐/延迟/P95 + `go test -race ./...`）

## Phase 1 验收检查项

- [ ] 核心 KV 语义正确（覆盖写、读、删、不存在 key 删除静默成功）
- [ ] 数据约束生效（key ≤256B、value ≤1MB、空 key 拒绝）
- [ ] TTL 秒级过期可用（Set 带 TTL → Get 过期返回不存在）
- [ ] maxmemory 拒写返回错误码 3001
- [ ] 并发安全（`go test -race ./...` 无报错）
- [ ] HTTP API 可访问（PUT/GET/DELETE /v1/key + GET /v1/health）
- [ ] CLI REPL 可交互（连接 kvd 执行 set/get/del）
- [ ] 统一响应格式与错误码符合设计

## Phase 2 验收检查项

- [ ] AOF 持久化（写入→重启→数据恢复）
- [ ] AOF Rewrite（超阈值自动压缩）
- [ ] RDB 快照（手动 + 自动规则触发）
- [ ] 双持久化协同（AOF 优先恢复、停机 flush）
- [ ] 监控统计 API + CLI 可用
- [ ] 压测结果（吞吐/延迟/P95）
- [ ] race 模式全量回归通过
