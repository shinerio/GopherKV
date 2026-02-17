# GopherKV 系统设计文档

## 1. 技术架构设计 (Architecture)

系统采用经典的 **Client-Server 分层架构**。服务端 (kvd) 负责数据存储与持久化，客户端 (kvcli) 通过 HTTP RESTful API 与服务端交互。系统设计注重**高并发读写性能**与**数据一致性**。

### 关键技术选型

技术栈：go 1.26

| 维度 | 选型方案                  | 设计理由                                                                       |
| --- |-----------------------|----------------------------------------------------------------------------|
| **并发控制** | **Sharded Lock (分片锁)** | 将存储空间划分为 N 个分片（如 256 个），每个分片独立持有 `sync.RWMutex`，将锁竞争降低 1/N，显著提升高并发下的写入吞吐量。 |
| **数据存储** | `map[string][]byte`   | 使用 `[]byte` 而不是 `interface{}`，避免 GC 扫描压力，且利于直接进行 IO 持久化，减少序列化开销。           |
| **过期策略** | **惰性删除 + 小顶堆 (Min-Heap)** | 查询时检查是否过期（惰性）；后台维护最小堆，仅处理堆顶最近过期的 Key，避免全表扫描消耗 CPU。                         |
| **持久化** | **AOF + RDB 双持久化**     | AOF 顺序追加写日志保证高性能；定期触发 Rewrite 压缩冗余操作；RDB 提供全量快照用于快速恢复。                       |
| **通信协议** | `HTTP` + `JSON`       | 保持接口简单、易调试。Handler 层预留接口，未来可无缝切换至 Protobuf 或 RESP 协议。                      |

## 2. 核心组件设计

为了支撑 CLI 和 Server 的架构，我们将逻辑组件划分为以下 5 个核心模块：

### 2.1 存储引擎组件 (Storage Engine)

这是最底层的数据结构层，不依赖网络，仅关注内存管理和磁盘 I/O。

1. **ConcurrentDict (分片内存存储)**
* **核心逻辑**：内部包含一组（如 256 个）`Shard`。通过 `hash(key) % shard_count` 定位分片。
* **数据结构**：每个 Shard 包含 `map[string]Entry` 和 `sync.RWMutex`。`Entry` 包含 `Value ([]byte)` 和 `ExpiresAt (int64)`。

2. **Persister (持久化管理器 - AOF)**
* **核心逻辑**：负责 AOF 日志的写入与重放。
* **fsync 策略**：由 OS 控制刷盘时机（性能优先）。
* **优化点**：实现 `Rewrite()` 方法。当 AOF 文件大小超过阈值（如 64MB）时，自动将当前内存快照写入临时文件并原子替换旧日志。
* **AOF Rewrite 并发安全**：
  - Rewrite 启动时创建增量缓冲区
  - 旧 AOF 继续接收新写入
  - 同时新写入也追加到增量缓冲区
  - Rewrite 完成后，将增量缓冲区追加到新文件，原子替换旧 AOF
* **损坏恢复**：逐行解析 AOF 文件，遇到无法解析的行则截断，加载已解析数据并记录警告日志。

3. **RDB Manager (快照管理器)**
* **RDB 格式**：将全量内存数据序列化为二进制文件（GOB 编码或自定义格式）。
* **触发条件**：可配置，如 `save 300 10`（300 秒内 10 次修改则触发）。
* **手动触发**：通过 `POST /v1/snapshot` 或 CLI `snapshot` 命令。
* **快照过程**：fork 思路不适用于 Go，改用加读锁逐分片序列化方式。
* **文件命名**：`dump-<timestamp>.rdb`，写入临时文件后原子 rename。
* **损坏恢复**：加载损坏前数据，记录警告日志。

### 2.2 调度核心组件 (Core Engine)

系统的"大脑"，负责协调存储、过期与系统信号。

1. **TTL Manager (过期协调器)**
* **核心逻辑**：维护一个 `PriorityQueue` (小顶堆)，存储 `{Key, ExpiresAt}`。
* **运行机制**：启动独立 Goroutine，`Peek` 堆顶元素。如果未到期，计算 `WaitTime` 并挂起；如果已到期，调用存储引擎删除并弹出堆顶。
* **懒清理策略**：
  - 弹出堆顶时，检查 key 在存储引擎中的 `ExpiresAt` 是否与堆条目一致
  - 如果不一致（说明 key 被更新过或已被惰性删除），丢弃该条目，继续弹出下一个
  - 对已有 TTL 的 key 重新 Set 时，只往堆中 push 新条目，不删除旧条目（懒清理策略天然处理）

2. **Coordinator (系统协调员)**
* **核心逻辑**：连接 HTTP 层与存储层。
* **优雅停机**：
  1. 监听 `SIGINT/SIGTERM` 信号
  2. 调用 `http.Server.Shutdown(ctx)` 停止接收新连接（已有连接等待完成）
  3. 停止 TTL Manager goroutine
  4. 触发一次 RDB 快照（如果启用）
  5. 调用 `Persister.Sync()` 强制刷盘 AOF
  6. 关闭存储引擎
  7. 整个停机流程有 `shutdown_timeout` 超时保护

### 2.3 服务端通讯组件 (Server Transport)

负责网络协议解析与封装。

1. **HTTP Handler**
* **核心逻辑**：解析 URL 参数和 Body。将 HTTP 请求转换为 Core Engine 的方法调用。
* **Context 控制**：将 HTTP 的 `r.Context()` 传递给底层，支持请求超时取消。

2. **Response Responder**
* **核心逻辑**：统一响应格式 `{"code": 0, "data": ..., "msg": "ok"}`，处理业务错误码。

### 2.4 客户端 SDK (SDK Library)

1. **Client (SDK 入口)**
* **核心逻辑**：提供 `NewClient(options)`，封装 `http.Client`。
* **功能**：实现 `Set(ctx, key, val)`, `Get(ctx, key)` 等方法，自动处理重试逻辑和 JSON 反序列化。

### 2.5 CLI 交互组件 (Command Line UI)

位于 `cmd/kvcli`，面向终端用户。

1. **Command Executor**
* **核心逻辑**：解析命令行参数。
* **交互模式**：支持 `Interactive Mode` (REPL)，允许用户连续输入指令。
* **连接参数**：`kvcli -h <host> -p <port>`
* **命令格式**：

```
set <key> <value> [ttl <seconds>]    # 设置 key-value，可选 TTL
get <key>                             # 获取 value
del <key>                             # 删除 key
exists <key>                          # 检查 key 是否存在
ttl <key>                             # 查询剩余 TTL
stats                                 # 查看服务器统计
snapshot                              # 手动触发 RDB 快照
help                                  # 显示帮助
exit / quit                           # 退出
```

## 3. 代码目录结构 (Project Structure)

```text
gopher-kv/
├── cmd/
│   ├── kvd/                  # [Main] 服务端守护进程
│   │   └── main.go           # 负责初始化 Config、Engine、Server 并启动
│   └── kvcli/                # [Main] 命令行客户端
│       └── main.go           # 解析 Flag，启动交互式 Shell
├── internal/
│   ├── core/                 # 核心业务逻辑
│   │   ├── service.go        # 协调器 (Coordinator)
│   │   └── ttl_heap.go       # 过期时间堆实现
│   ├── storage/              # 物理存储层
│   │   ├── concurrent_map.go # 分片锁 Map 实现
│   │   └── persistence.go    # AOF 读写与 Rewrite 逻辑
│   └── server/               # 网络接入层
│       ├── http_handler.go   # HTTP 路由与处理
│       └── response.go       # 统一响应封装
├── pkg/
│   ├── client/               # 公共 SDK (允许外部项目 Import)
│   │   └── client.go
│   ├── protocol/             # 协议定义 (DTOs, ErrorCodes)
│       └── types.go
│   └── utils/                # 通用工具 (Hash, Time)
├── configs/                  # 配置文件模板
│   └── config.yaml           # 定义端口、数据路径、分片数等
├── Makefile                  # 自动化构建 (build, test, clean)
├── go.mod
├── docs                      # 项目文档
└── README.md
```

## 4. 配置文件定义 (Configuration)

```yaml
# config/config.yaml
server:
  port: 6380
  read_timeout: 5s
  write_timeout: 5s
  shutdown_timeout: 30s

storage:
  shard_count: 256
  max_key_size: 256        # bytes
  max_value_size: 1048576  # 1MB
  max_memory: 268435456    # 256MB

aof:
  enabled: true
  file_path: "./data/appendonly.aof"
  rewrite_threshold: 67108864  # 64MB

rdb:
  enabled: true
  file_path: "./data/dump.rdb"
  save_rules:              # N秒内M次修改触发快照
    - seconds: 900
      changes: 1
    - seconds: 300
      changes: 10
    - seconds: 60
      changes: 10000

log:
  level: "info"            # debug/info/warn/error
```

## 5. 接口设计 (API Design)

### HTTP 接口

* **PUT /v1/key** — 设置 key-value
  * Body: `{"key": "user:1001", "value": "base64_data", "ttl": 3600}`
  * Response: `{"code": 0, "data": null, "msg": "ok"}`

* **GET /v1/key?k=user:1001** — 获取 value
  * Response: `{"code": 0, "data": {"value": "base64_data", "ttl_remaining": 3590}, "msg": "ok"}`

* **DELETE /v1/key?k=user:1001** — 删除 key
  * Response: `{"code": 0, "data": null, "msg": "ok"}`

* **GET /v1/stats** — 返回监控统计数据
  * Response: `{"code": 0, "data": {"keys": 1024, "memory": 10485760, "hits": 5000, "misses": 200, "requests": {"set": 3000, "get": 5200, "del": 100}, "uptime": 86400}, "msg": "ok"}`

* **POST /v1/snapshot** — 手动触发 RDB 快照
  * Response: `{"code": 0, "data": {"status": "ok", "path": "data/dump-1700000000.rdb"}, "msg": "ok"}`

* **GET /v1/health** — 健康检查
  * Response: `{"code": 0, "data": {"status": "healthy"}, "msg": "ok"}`

### 错误响应规范

统一错误响应格式：`{"code": <错误码>, "data": null, "msg": "<错误描述>"}`

| 错误码 | 含义 | HTTP Status |
|--------|------|-------------|
| 0 | 成功 | 200 |
| 1001 | Key 不存在 | 404 |
| 1002 | Key 已过期 | 404 |
| 2001 | Key 超长（>256B） | 400 |
| 2002 | Value 超大（>1MB） | 400 |
| 2003 | 请求参数无效 | 400 |
| 3001 | 内存已满，拒绝写入 | 507 |
| 5001 | 服务内部错误 | 500 |

### 内部 Go 接口定义 (Storage)

```go
// Storage 定义底层存储引擎的行为
type Storage interface {
    // Set 写入数据，ttl=0 表示不过期
    Set(key string, value []byte, ttl time.Duration) error

    // Get 读取数据，返回 value 和是否存在
    Get(key string) ([]byte, bool)

    // Delete 删除数据
    Delete(key string) error

    // Exists 判断 key 是否存在
    Exists(key string) bool

    // TTL 查询剩余 TTL
    TTL(key string) (time.Duration, bool)

    // Keys 返回 key 总数（用于监控）
    Keys() int

    // MemUsage 返回估算内存占用（用于监控和内存限制）
    MemUsage() int64

    // Close 优雅关闭，触发落盘
    Close() error
}
```

## 6. 持久化协议 (Persistence Protocol)

### AOF 文本协议格式

每行一条命令，字段用 `\t` 分隔：

```
SET\t<key>\t<base64_value>\t<expires_at_unix>\n
DEL\t<key>\n
```

示例：

```
SET	user:1001	aGVsbG8=	1700000000
DEL	user:1002
```

- Value 使用 base64 编码避免特殊字符问题
- `expires_at` 使用 Unix 时间戳，0 表示不过期
- 损坏恢复策略：逐行解析，遇到无法解析的行则截断，加载已解析数据并记录警告