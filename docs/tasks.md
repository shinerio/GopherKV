# GopherKV Tasks

## 进度总览

- 总体进度：`16 / 18`（约 `89%`）
- MVP 进度：`9 / 9`（`100%`）
- 高级能力进度：`7 / 9`（`78%`）

## Phase 1: MVP（最小可测试闭环）

- [x] T1 项目骨架与配置加载（`cmd/internal/pkg/configs` 结构、默认配置、文件加载）
- [x] T2 协议与错误码（统一响应结构 + 错误码映射）
- [x] T3 分片并发存储引擎（`RWMutex` + shard map）
- [x] T4 数据约束与内存上限（key/value 校验 + maxmemory 拒写）
- [x] T5 TTL（惰性删除 + 后台过期清理）
- [x] T6 HTTP Server（`PUT/GET/DELETE /v1/key` + `GET /v1/health`）
- [x] T7 CLI（REPL + `set/get/del/exists/ttl/help/exit`）
- [x] T8 日志与优雅停机框架（`slog` + shutdown 流程）
- [x] T9 MVP 测试与验收（`go test ./...` 通过，含核心存储单测）

## Phase 2: 高级能力补齐

- [x] T10 AOF 持久化（append/replay/损坏截断恢复）
- [x] T11 AOF Rewrite（阈值检测 + rewrite 流程 + 增量缓冲）
- [x] T12 RDB 快照（手动触发 + 自动规则触发 + 文件落盘）
- [x] T13 双持久化协同（AOF 优先恢复、停机 flush/snapshot）
- [x] T14 监控统计（keys/memory/hits/misses/requests/uptime + stats API）
- [x] T15 SDK Client（set/get/del/exists/ttl/stats/snapshot）
- [x] T16 安全扩展点（HTTP middleware hook 预留）
- [x] T17 配置与文档收敛（`configs/config.yaml` + `README.md` 更新）
- [ ] T18 全量回归与性能验收（缺压测脚本与基线报告）

## 验收检查项

- [x] 核心 KV 语义符合要求（覆盖写、读、删、不存在删除）
- [x] 数据约束生效（key/value 限制）
- [x] TTL 秒级过期可用
- [x] maxmemory 拒写返回错误
- [x] 并发安全（无编译错误，核心单测通过）
- [x] AOF + RDB 双持久化实现
- [x] API + CLI 可访问
- [x] 统一响应与错误码
- [ ] 压测结果（吞吐/延迟/P95）
- [ ] race 模式回归（`go test -race ./...`）

## 下一步（收尾）

- [ ] N1 补充 `-race` 回归并记录结果
- [ ] N2 增加基础压测脚本（并发读写）与基线数据
- [ ] N3 补充 HTTP/CLI 集成测试（端到端）
