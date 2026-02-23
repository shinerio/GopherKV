# GopherKV

一个轻量级、高性能、支持持久化的键值存储系统，用 Go 语言实现。

## 功能特性

- ✅ **基础 KV 操作**: Set/Get/Delete/Exists
- ✅ **TTL 过期机制**: 支持为键设置生存时间
- ✅ **分片并发存储**: 256 分片设计，高并发读写安全
- ✅ **数据约束**: Key ≤256B，Value ≤1MB
- ✅ **内存管理**: 可配置 maxmemory 上限
- ✅ **AOF 持久化**: append/replay/损坏截断恢复 + rewrite
- ✅ **RDB 快照**: 手动触发与自动规则触发
- ✅ **HTTP API**: RESTful 接口
- ✅ **CLI 工具**: 交互式命令行
- ✅ **GUI 工具**: Windows 桌面图形界面 (Wails v2 + WebView2)
- ✅ **监控统计**: `GET /v1/stats` + CLI `stats` + GUI dashboard
- ✅ **安全扩展点**: HTTP middleware hook 预留
- ✅ **优雅停机**: 信号处理与资源清理

## 快速开始

### 构建

#### Windows
```powershell
.\build.bat
```

#### Linux/macOS
```bash
make build
```

### 运行服务端

```bash
# 使用默认配置
bin/kvd

# 指定配置文件
bin/kvd -config configs/config.yaml
```

### 运行 CLI 客户端

```bash
bin/kvcli -h localhost -p 6380
```

### 构建 GUI 客户端 (Windows)

需先安装 [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

然后构建：

```powershell
# Windows
.\build.bat gui

# Linux/macOS
make gui
```

运行生成的 `bin/kvgui.exe`（与 `bin/kvd.exe` / `bin/kvcli.exe` 同目录），无需额外安装（Windows 11 自带 WebView2；Windows 10 首次运行自动安装）。

```powershell
bin\kvgui.exe
```

开发模式（热重载）：

```powershell
cd cmd/kvgui
wails dev
```

## 使用示例

### CLI 命令

```
# 连接到服务器
bin/kvcli -h localhost -p 6380

# 设置键值对
set mykey "hello world"

# 设置带 TTL 的键（10秒过期）
set tempkey "expires soon" ttl 10

# 获取值
get mykey

# 检查键是否存在
exists mykey

# 删除键
del mykey

# 查看帮助
help

# 查看统计
stats

# 手动触发快照
snapshot

# 退出
exit
```

### HTTP API

#### 健康检查
```bash
curl http://localhost:6380/v1/health
```

#### 设置键值
```bash
curl -X PUT http://localhost:6380/v1/key \
  -H "Content-Type: application/json" \
  -d '{"key": "mykey", "value": "aGVsbG8=", "ttl": 3600}'
```
注：value 需要 base64 编码

#### 获取键值
```bash
curl "http://localhost:6380/v1/key?k=mykey"
```

#### 删除键
```bash
curl -X DELETE "http://localhost:6380/v1/key?k=mykey"
```

#### 查看统计
```bash
curl http://localhost:6380/v1/stats
```

#### 手动触发快照
```bash
curl -X POST http://localhost:6380/v1/snapshot
```

## 配置文件

参考 `configs/config.yaml`:

```yaml
server:
  port: 6380
  read_timeout: 5s
  write_timeout: 5s
  shutdown_timeout: 30s

storage:
  shard_count: 256
  max_key_size: 256
  max_value_size: 1048576
  max_memory: 268435456

aof:
  enabled: true
  file_path: "./data/appendonly.aof"
  rewrite_threshold: 67108864

rdb:
  enabled: true
  file_path: "./data/dump.rdb"
  save_rules:
    - seconds: 900
      changes: 1
    - seconds: 300
      changes: 10
    - seconds: 60
      changes: 10000

log:
  level: "info"
```

## 项目结构

```
gopher-kv/
├── cmd/
│   ├── kvd/          # 服务端守护进程
│   ├── kvcli/        # 命令行客户端
│   └── kvgui/        # 桌面 GUI 客户端 (Wails v2)
│       ├── main.go
│       ├── app.go
│       ├── frontend/ # HTML/CSS/JS 前端
│       └── wails.json
├── internal/
│   ├── core/         # 核心业务逻辑
│   ├── storage/      # 存储引擎
│   ├── server/       # HTTP 服务
│   └── pkg/configs/  # 配置管理
├── pkg/
│   ├── client/       # SDK 客户端
│   └── protocol/     # 协议与错误码
├── configs/          # 配置文件
├── bin/              # 编译产物
└── docs/             # 项目文档
```

## 开发

### 运行测试
```bash
# 所有测试
make test

# Race 检测
make race
```

### 清理
```bash
make clean
```

## License

MIT
