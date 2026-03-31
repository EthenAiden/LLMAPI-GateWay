# ai-ide-gateway

AI IDE 模型 API 反代与治理网关 - 基于 Go 的高性能网关系统，为 Cursor/Kiro/Antigravity 等 AI IDE 提供统一的 OpenAI 兼容接口。

## 功能特性

- **多协议适配**: 支持 Cursor/Kiro/Antigravity 协议转换为 OpenAI 标准接口
- **动态路由**: 基于 Redis 的动态路由规则，支持秒级更新
- **故障切换**: 三级故障切换机制（备用 Key → 替代模型 → 兜底模型）
- **熔断限流**: 滑动窗口熔断器 + Redis Lua 分布式限流
- **流式代理**: SSE 流式透传，支持实时 Token 计量
- **配额管理**: 三级配额树（用户/应用/模型）+ 预扣结算机制
- **流量回放**: Kafka 异步采集，支持历史请求回放与对比
- **可观测性**: Prometheus 监控指标 + 结构化日志

## 快速开始

### 前置要求

- Go 1.21+
- Redis 6.0+
- PostgreSQL 13+
- Kafka 2.8+ (可选，用于流量回放)

### 安装依赖

```bash
go mod download
```

### 配置

复制配置模板并修改：

```bash
cp configs/config.example.yaml configs/config.yaml
```

### 运行

```bash
go run cmd/gateway/main.go
```

## 项目结构

```
.
├── cmd/                    # 应用入口
│   └── gateway/           # 网关主程序
├── internal/              # 私有应用代码
│   ├── adapter/          # 协议适配器
│   ├── router/           # 路由管理
│   ├── circuit/          # 熔断器
│   ├── ratelimit/        # 限流器
│   ├── proxy/            # SSE 代理
│   ├── quota/            # 配额管理
│   └── replay/           # 流量回放
├── pkg/                   # 公共库代码
│   ├── logger/           # 日志工具
│   └── metrics/          # 监控指标
└── configs/               # 配置文件
```

## API 文档

### 聊天补全接口

```bash
POST /v1/chat/completions
Authorization: Bearer YOUR_API_KEY

{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": true
}
```

## 开发

### 运行测试

```bash
go test ./...
```

### 构建

```bash
go build -o bin/gateway cmd/gateway/main.go
```

## License

MIT
