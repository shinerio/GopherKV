# GopherKV

一个轻量级、高性能、支持持久化的键值存储系统，用 Go 语言实现。

## 功能特性

- ✅ **基础 KV 操作**: Set/Get/Delete/Exists
- ✅ **TTL 过期机制**: 支持为键设置生存时间
- ✅ **分片并发存储**: 256 分片设计，高并发读写安全
- ✅ **数据约束**: Key ≤256B，Value ≤1MB
- ✅ **内存管理**: 可配置 maxmemory 上限
- ✅ **HTTP API**: RESTful 接口
- ✅ **CLI 工具**: 交互式命令行
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
bin/kvd -config config/config.yaml
```

### 运行 CLI 客户端

```bash
bin/kvcli -h localhost -p 6380
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

log:
  level: "info"
```

## 项目结构

```
gopher-kv/
├── cmd/
│   ├── kvd/          # 服务端守护进程
│   └── kvcli/        # 命令行客户端
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
