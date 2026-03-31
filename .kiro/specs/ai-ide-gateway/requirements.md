# Requirements Document

## Introduction

ai-ide-gateway 是一个基于 Go 的 AI IDE 模型 API 反代与治理网关项目。针对 Cursor / Kiro / Antigravity 等 AI IDE 内置模型无独立 API、多模型切换成本高、调用不稳定等问题，设计并实现模型 API 反代与治理网关，对外暴露 OpenAI 兼容接口。

## Glossary

- **Gateway**: 网关系统，负责接收客户端请求并转发到上游 AI IDE 服务
- **Upstream_Provider**: 上游服务提供商（Cursor、Kiro、Antigravity）
- **Route_Rule**: 路由规则，定义请求如何分发到不同的上游服务
- **Circuit_Breaker**: 熔断器，当错误率超过阈值时自动切断流量
- **Rate_Limiter**: 限流器，控制请求速率防止过载
- **Token_Quota**: Token 配额，用户/应用/模型的使用额度
- **SSE_Stream**: Server-Sent Events 流式传输协议
- **Failover**: 故障切换，当主服务不可用时自动切换到备用服务
- **Health_Probe**: 健康探测，周期性检查服务可用性
- **Replay_System**: 流量回放系统，记录并重放历史请求

## Requirements

### Requirement 1: 多协议适配

**User Story:** 作为开发者，我希望网关能够适配多种 AI IDE 的非标协议，以便我可以使用统一的 OpenAI 接口调用所有模型。

#### Acceptance Criteria

1. THE Gateway SHALL 支持 Cursor 协议的请求解析和响应转换
2. THE Gateway SHALL 支持 Kiro 协议的请求解析和响应转换
3. THE Gateway SHALL 支持 Antigravity 协议的请求解析和响应转换
4. THE Gateway SHALL 对外暴露 OpenAI 兼容的 /v1/chat/completions 接口
5. WHEN 接收到 OpenAI 格式请求时，THE Gateway SHALL 根据模型标识自动选择对应的 Upstream_Provider 协议
6. WHEN 协议转换失败时，THE Gateway SHALL 返回标准的 OpenAI 错误格式响应

### Requirement 2: 动态路由与故障切换

**User Story:** 作为系统管理员，我希望路由规则可以动态更新并秒级生效，以便快速响应服务变化和故障。

#### Acceptance Criteria

1. THE Gateway SHALL 将 Route_Rule 持久化存储到 Redis
2. WHEN Route_Rule 更新时，THE Gateway SHALL 通过 Redis Pub/Sub 推送变更通知
3. WHEN 接收到路由变更通知时，THE Gateway SHALL 在 1 秒内加载新规则
4. WHEN 同一模型配置多个 API Key 时，THE Gateway SHALL 支持轮询负载均衡
5. WHEN 主 API Key 调用失败时，THE Gateway SHALL 自动切换到同模型的备用 Key
6. WHEN 同模型所有 Key 均不可用时，THE Gateway SHALL 切换到同能力的替代模型
7. WHEN 所有替代模型均不可用时，THE Gateway SHALL 使用配置的兜底模型
8. THE Gateway SHALL 每 30 秒执行一次 Health_Probe 检测上游服务可用性
9. WHEN Health_Probe 检测到服务恢复时，THE Gateway SHALL 自动将节点加回路由池

### Requirement 3: 熔断与限流

**User Story:** 作为系统管理员，我希望系统具备熔断和限流能力，以便保护上游服务和保证服务质量。

#### Acceptance Criteria

1. THE Gateway SHALL 使用滑动窗口算法统计每个 Upstream_Provider 的错误率
2. WHEN 错误率在 10 秒窗口内超过 50% 时，THE Circuit_Breaker SHALL 触发熔断
3. WHEN Circuit_Breaker 处于熔断状态时，THE Gateway SHALL 拒绝新请求并返回 503 错误
4. WHEN Circuit_Breaker 熔断 30 秒后，THE Gateway SHALL 进入半开状态允许探测请求
5. WHEN 半开状态下探测请求成功时，THE Circuit_Breaker SHALL 恢复正常状态
6. THE Gateway SHALL 使用 Redis Lua 脚本实现分布式令牌桶限流
7. THE Rate_Limiter SHALL 支持按用户、应用、模型三个维度独立限流
8. WHEN 限流拒绝请求时，THE Gateway SHALL 在 2 毫秒内返回 429 错误（P99 延迟）

### Requirement 4: SSE 流式代理

**User Story:** 作为开发者，我希望网关支持流式响应，以便实现实时的对话体验。

#### Acceptance Criteria

1. THE Gateway SHALL 使用 io.Pipe 和 goroutine 实现流式透传
2. THE Gateway SHALL 不缓冲完整响应内容到内存
3. WHEN 接收到上游 SSE 数据块时，THE Gateway SHALL 实时解析 Token 使用量
4. WHEN 客户端断开连接时，THE Gateway SHALL 立即关闭上游连接防止泄漏
5. WHEN 上游连接异常时，THE Gateway SHALL 向客户端发送错误事件并关闭流
6. WHEN 处理 1000 并发长连接时，THE Gateway SHALL 保证首 Token 延迟增量小于 15 毫秒

### Requirement 5: Token 计量与配额管理

**User Story:** 作为系统管理员，我希望系统能够精确计量 Token 使用量并管理配额，以便控制成本和防止滥用。

#### Acceptance Criteria

1. THE Gateway SHALL 维护用户、应用、模型三级 Token_Quota 树结构
2. THE Gateway SHALL 使用 Redis Lua 脚本原子性扣减配额
3. WHEN 处理非流式请求时，THE Gateway SHALL 在响应返回后一次性扣减 Token
4. WHEN 处理流式请求时，THE Gateway SHALL 在流开始时预扣估算的 Token 数量
5. WHEN 流式请求结束时，THE Gateway SHALL 根据实际消耗校正配额（多退少补）
6. WHEN Token_Quota 不足时，THE Gateway SHALL 拒绝请求并返回 402 错误
7. THE Gateway SHALL 记录每次 Token 扣减的详细日志（用户、应用、模型、数量、时间戳）

### Requirement 6: 流量回放

**User Story:** 作为开发者，我希望系统能够记录和回放历史请求，以便进行模型效果对比和问题排查。

#### Acceptance Criteria

1. THE Gateway SHALL 异步采集请求和响应元数据到 Kafka
2. THE Gateway SHALL 在主请求路径中避免任何磁盘 IO 操作
3. THE Gateway SHALL 记录请求的时间戳、用户标识、模型名称、Prompt、响应内容、Token 用量
4. THE Replay_System SHALL 支持按时间范围筛选历史请求
5. THE Replay_System SHALL 支持按模型名称筛选历史请求
6. THE Replay_System SHALL 支持按用户标识筛选历史请求
7. THE Replay_System SHALL 支持将历史请求重新发送到指定模型
8. THE Replay_System SHALL 支持对比原始响应和回放响应的差异

### Requirement 7: 配置管理与监控

**User Story:** 作为系统管理员，我希望能够方便地管理配置并监控系统运行状态，以便快速定位和解决问题。

#### Acceptance Criteria

1. THE Gateway SHALL 支持通过配置文件初始化路由规则和限流策略
2. THE Gateway SHALL 提供 HTTP API 用于动态更新路由规则
3. THE Gateway SHALL 提供 HTTP API 用于查询当前路由状态和健康状态
4. THE Gateway SHALL 暴露 Prometheus 格式的监控指标
5. THE Gateway SHALL 记录请求总数、错误数、延迟分布、熔断次数、限流次数等指标
6. THE Gateway SHALL 记录每个 Upstream_Provider 的调用次数和错误率
7. THE Gateway SHALL 提供结构化日志输出（JSON 格式）

### Requirement 8: 性能与可靠性

**User Story:** 作为系统管理员，我希望网关具备高性能和高可靠性，以便支撑生产环境的大规模使用。

#### Acceptance Criteria

1. WHEN 执行限流拒绝时，THE Gateway SHALL 保证 P99 延迟小于 2 毫秒
2. WHEN 处理 1000 并发 SSE 长连接时，THE Gateway SHALL 保证首 Token 延迟增量小于 15 毫秒
3. WHEN Route_Rule 更新时，THE Gateway SHALL 在 1 秒内完成规则加载
4. THE Gateway SHALL 在主请求路径中避免任何磁盘 IO 操作
5. THE Gateway SHALL 支持水平扩展（多实例部署）
6. WHEN 单个实例崩溃时，THE Gateway SHALL 不影响其他实例的服务
7. THE Gateway SHALL 支持优雅关闭（等待现有请求完成后再退出）

### Requirement 9: 安全与认证

**User Story:** 作为系统管理员，我希望网关具备完善的安全机制，以便保护 API 不被未授权访问。

#### Acceptance Criteria

1. THE Gateway SHALL 支持 API Key 认证
2. WHEN 请求缺少 Authorization 头时，THE Gateway SHALL 返回 401 错误
3. WHEN API Key 无效时，THE Gateway SHALL 返回 403 错误
4. THE Gateway SHALL 支持为不同 API Key 配置不同的 Token_Quota
5. THE Gateway SHALL 记录每个 API Key 的使用情况
6. THE Gateway SHALL 支持 API Key 的创建、更新、删除操作
7. THE Gateway SHALL 对敏感配置（如上游 API Key）进行加密存储
