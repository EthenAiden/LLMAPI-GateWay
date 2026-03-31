# Implementation Plan: ai-ide-gateway

## Overview

本实施计划将 ai-ide-gateway 项目分解为可独立完成和测试的任务。项目采用 Go 语言实现，基于设计文档中的架构和技术方案，按照 10 个阶段逐步构建完整的 AI IDE 模型 API 反代与治理网关系统。

## Tasks

- [x] 1. 核心基础设施搭建
  - [x] 1.1 初始化 Go 项目结构和依赖管理
    - 创建标准 Go 项目目录结构（cmd、internal、pkg、configs）
    - 初始化 go.mod 并添加核心依赖（gin/echo、zap、redis、kafka、postgres）
    - 配置 .gitignore 和 README.md
    - _Requirements: 8.4_

  - [x] 1.2 实现配置管理模块
    - 创建 config 包，支持 YAML 配置文件加载
    - 实现配置结构体（Server、Redis、Kafka、Database、CircuitBreaker、RateLimiter）
    - 支持环境变量覆盖配置
    - _Requirements: 7.1_

  - [x] 1.3 实现日志模块
    - 集成 zap 日志库
    - 实现结构化 JSON 日志输出
    - 实现日志级别控制和采样
    - _Requirements: 7.7_

  - [x] 1.4 实现 HTTP 服务器框架
    - 创建 HTTP 服务器基础结构
    - 实现优雅关闭机制
    - 实现健康检查端点（/health、/ready）
    - _Requirements: 8.7_

  - [x] 1.5 集成存储层客户端
    - 集成 Redis 客户端并实现连接池
    - 集成 Kafka 生产者和消费者客户端
    - 集成 PostgreSQL 数据库驱动
    - 实现连接健康检查和重连机制
    - _Requirements: 2.1, 6.1_

- [x] 2. 协议适配层实现
  - [x] 2.1 设计适配器框架
    - 定义 ProtocolAdapter 接口（ConvertRequest、ConvertResponse、ConvertStreamChunk）
    - 实现适配器工厂模式（AdapterFactory）
    - 定义 OpenAI 和上游协议的数据模型
    - _Requirements: 1.4, 1.5_

  - [ ]* 2.2 编写协议转换往返一致性属性测试
    - **Property 1: 协议转换往返一致性**
    - **Validates: Requirements 1.1, 1.2, 1.3**

  - [x] 2.3 实现 Cursor 协议适配器
    - 实现 CursorAdapter 结构体
    - 实现 OpenAI → Cursor 请求转换逻辑
    - 实现 Cursor → OpenAI 响应转换逻辑
    - 实现 Cursor SSE 流数据块解析
    - 实现 Cursor Token 提取器
    - _Requirements: 1.1_

  - [ ]* 2.4 编写 Cursor 适配器单元测试
    - 测试基本请求转换
    - 测试边界情况（空消息、超长内容）
    - 测试错误处理
    - _Requirements: 1.1_

  - [x] 2.5 实现 Kiro 协议适配器
    - 实现 KiroAdapter 结构体
    - 实现 OAuth Token 管理器（刷新、缓存）
    - 实现 OpenAI → Kiro 请求转换逻辑
    - 实现 Kiro → OpenAI 响应转换逻辑
    - 实现 Kiro SSE 流数据块解析
    - 实现 Kiro Token 提取器
    - _Requirements: 1.2_

  - [ ]* 2.6 编写 Kiro 适配器单元测试
    - 测试 OAuth Token 刷新逻辑
    - 测试请求转换和响应转换
    - 测试 SSE 流解析
    - _Requirements: 1.2_

  - [x] 2.7 实现 Antigravity 协议适配器
    - 实现 AntigravityAdapter 结构体
    - 实现 OpenAI → Antigravity 请求转换逻辑
    - 实现 Antigravity → OpenAI 响应转换逻辑
    - 实现 Antigravity SSE 流数据块解析
    - 实现 Antigravity Token 提取器
    - _Requirements: 1.3_

  - [ ]* 2.8 编写 Antigravity 适配器单元测试
    - 测试请求转换和响应转换
    - 测试 SSE 流解析
    - _Requirements: 1.3_

  - [x] 2.9 实现协议转换错误处理
    - 实现统一的错误响应格式（OpenAI 标准）
    - 实现协议转换失败的错误处理
    - _Requirements: 1.6_

- [x] 3. Checkpoint - 验证协议适配层
  - 确保所有协议适配器测试通过，询问用户是否有问题

- [x] 4. 路由与故障切换实现
  - [x] 4.1 实现路由规则数据模型
    - 定义 RouteRule、APIKeyConfig、FallbackConfig 结构体
    - 实现路由规则的 JSON 序列化/反序列化
    - _Requirements: 2.1_

  - [x] 4.2 实现路由管理器
    - 实现 RouteManager 结构体
    - 实现从 Redis 加载路由规则（LoadRules）
    - 实现 Redis Pub/Sub 订阅路由更新（SubscribeUpdates）
    - 实现路由规则热加载（1 秒内生效）
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 4.3 实现负载均衡算法
    - 实现轮询算法（roundRobinSelect）
    - 实现随机算法（randomSelect）
    - 实现加权轮询算法（weightedSelect）
    - 实现 API Key 选择逻辑（SelectAPIKey）
    - _Requirements: 2.4_

  - [ ]* 4.4 编写负载均衡属性测试
    - **Property 6: 轮询负载均衡公平性**
    - **Validates: Requirements 2.4**

  - [x] 4.5 实现三级故障切换逻辑
    - 实现 FailoverExecutor 结构体
    - 实现 Level 1：尝试主 API Key（tryPrimaryKey）
    - 实现 Level 2：尝试同模型备用 Key（tryBackupKeys）
    - 实现 Level 3：尝试替代模型（tryAlternativeModels）
    - 实现 Level 4：使用兜底模型（tryFallbackModel）
    - _Requirements: 2.5, 2.6, 2.7_

  - [ ]* 4.6 编写故障切换属性测试
    - **Property 7: 故障切换链完整性**
    - **Validates: Requirements 2.5, 2.6, 2.7**

  - [x] 4.7 实现健康探测机制
    - 实现 HealthProbe 结构体
    - 实现周期性健康检查（每 30 秒）
    - 实现失败节点的自动恢复检测
    - 实现节点状态更新（Active/Cooldown/Failed）
    - _Requirements: 2.8, 2.9_

  - [ ]* 4.8 编写健康探测单元测试
    - 测试健康检查逻辑
    - 测试节点自动恢复
    - _Requirements: 2.8, 2.9_

- [x] 5. 熔断与限流实现
  - [x] 5.1 实现滑动窗口数据结构
    - 实现 SlidingWindow 结构体
    - 实现时间桶（Bucket）数据结构
    - 实现滑动窗口记录逻辑（record）
    - 实现错误率计算（calculateErrorRate）
    - _Requirements: 3.1_

  - [ ]* 5.2 编写滑动窗口属性测试
    - **Property 9: 滑动窗口错误率计算**
    - **Validates: Requirements 3.1**

  - [x] 5.3 实现熔断器状态机
    - 实现 CircuitBreaker 结构体
    - 实现三种状态（Closed、Open、HalfOpen）
    - 实现状态转换逻辑
    - 实现请求允许检查（AllowRequest）
    - 实现成功/失败记录（RecordSuccess、RecordFailure）
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

  - [ ]* 5.4 编写熔断器属性测试
    - **Property 10: 熔断器触发条件**
    - **Property 11: 熔断状态拒绝请求**
    - **Property 12: 熔断器半开状态恢复**
    - **Validates: Requirements 3.2, 3.3, 3.5**

  - [x] 5.5 实现 Redis Lua 限流脚本
    - 编写令牌桶算法 Lua 脚本（tokenBucketScript）
    - 实现令牌生成和消费逻辑
    - 实现过期时间设置
    - _Requirements: 3.6_

  - [x] 5.6 实现分布式限流器
    - 实现 RateLimiter 结构体
    - 实现单维度限流检查（AllowRequest）
    - 实现多维度限流检查（CheckMultiDimension）
    - 实现限流错误处理（RateLimitError）
    - _Requirements: 3.6, 3.7, 3.8_

  - [ ]* 5.7 编写限流器属性测试
    - **Property 13: 多维度限流独立性**
    - **Validates: Requirements 3.7**

  - [ ]* 5.8 编写限流器性能测试
    - 测试 P99 延迟 < 2ms
    - _Requirements: 3.8, 8.1_

- [x] 6. Checkpoint - 验证熔断与限流
  - 确保所有熔断和限流测试通过，询问用户是否有问题

- [x] 7. SSE 流式代理实现
  - [x] 7.1 实现流式代理核心逻辑
    - 实现 StreamProxy 结构体
    - 实现 io.Pipe 流式透传（ProxyStream）
    - 实现 SSE 响应头设置
    - 实现上游请求发送和响应读取
    - _Requirements: 4.1, 4.2_

  - [x] 7.2 实现 Token 实时解析
    - 实现 TokenParser 结构体
    - 实现 TokenExtractor 接口
    - 实现各协议的 Token 提取器（CursorTokenExtractor、KiroTokenExtractor、AntigravityTokenExtractor）
    - 实现 Token 累加计数器
    - _Requirements: 4.3_

  - [ ]* 7.3 编写 Token 解析属性测试
    - **Property 14: SSE Token 实时解析**
    - **Validates: Requirements 4.3**

  - [x] 7.4 实现连接管理器
    - 实现 ConnectionManager 结构体
    - 实现连接注册和注销（Register、Close）
    - 实现连接超时清理（cleanupStaleConnections）
    - 实现周期性清理任务（StartCleanup）
    - _Requirements: 4.4_

  - [x] 7.5 实现客户端断开检测
    - 实现 CloseNotifier 监听
    - 实现客户端断开时的连接清理
    - _Requirements: 4.4_

  - [ ]* 7.6 编写连接管理属性测试
    - **Property 15: 客户端断开连接清理**
    - **Validates: Requirements 4.4**

  - [x] 7.7 实现上游异常处理
    - 实现上游连接异常检测
    - 实现 SSE 错误事件发送
    - 实现流关闭逻辑
    - _Requirements: 4.5_

  - [ ]* 7.8 编写流式代理性能测试
    - 测试 1000 并发长连接首 Token 延迟增量 < 15ms
    - _Requirements: 4.6, 8.2_

- [x] 8. Token 计量与配额管理实现
  - [x] 8.1 实现三级配额树数据结构
    - 实现 QuotaTree 结构体
    - 实现 QuotaNode 数据模型（User、App、Model 三级）
    - 实现配额节点 CRUD 操作
    - 实现配额层级关系维护
    - _Requirements: 5.1_

  - [ ]* 8.2 编写配额树属性测试
    - **Property 17: 配额树层级完整性**
    - **Validates: Requirements 5.1**

  - [x] 8.3 实现 Redis Lua 配额脚本
    - 编写配额预扣 Lua 脚本（reserveQuotaScript）
    - 编写配额结算 Lua 脚本（settleQuotaScript）
    - 实现三级配额原子性扣减
    - _Requirements: 5.2_

  - [x] 8.4 实现配额管理器
    - 实现 QuotaManager 结构体
    - 实现配额预扣逻辑（ReserveQuota）
    - 实现配额结算逻辑（SettleQuota）
    - 实现多退少补机制（deductAdditional、refund）
    - 实现预扣记录管理（QuotaReservation）
    - _Requirements: 5.2, 5.4, 5.5_

  - [ ]* 8.5 编写配额管理属性测试
    - **Property 18: 配额扣减原子性**
    - **Property 20: 流式请求配额预扣**
    - **Property 21: 流式请求配额结算往返**
    - **Validates: Requirements 5.2, 5.4, 5.5**

  - [x] 8.6 实现非流式请求配额扣减
    - 实现响应完成后的一次性配额扣减
    - _Requirements: 5.3_

  - [x] 8.7 实现配额不足检查
    - 实现配额不足拒绝逻辑
    - 实现 402 错误响应
    - _Requirements: 5.6_

  - [ ]* 8.8 编写配额不足属性测试
    - **Property 22: 配额不足拒绝**
    - **Validates: Requirements 5.6**

  - [x] 8.9 实现 Token 扣减日志
    - 实现详细的配额扣减日志记录
    - 记录用户、应用、模型、数量、时间戳
    - _Requirements: 5.7_

- [x] 9. Checkpoint - 验证配额管理
  - 确保所有配额管理测试通过，询问用户是否有问题

- [x] 10. 流量回放实现
  - [x] 10.1 实现 Kafka 异步采集
    - 实现 ReplayCollector 结构体
    - 实现 RequestMetadata 数据模型
    - 实现异步元数据采集逻辑（Collect）
    - 实现 Kafka 生产者错误处理
    - _Requirements: 6.1, 6.2_

  - [ ]* 10.2 编写元数据采集属性测试
    - **Property 24: 请求元数据异步采集**
    - **Property 25: 元数据记录完整性**
    - **Validates: Requirements 6.1, 6.3**

  - [x] 10.3 实现元数据存储
    - 实现 ReplayStorage 结构体
    - 实现 Kafka 消费者逻辑（StartConsumer）
    - 实现数据库存储逻辑（processMessage）
    - 创建 request_history 表结构
    - _Requirements: 6.3_

  - [x] 10.4 实现历史查询 API
    - 实现 QueryHistory 方法
    - 实现多条件过滤（时间范围、用户、模型）
    - 实现分页查询
    - _Requirements: 6.4, 6.5, 6.6_

  - [ ]* 10.5 编写历史查询属性测试
    - **Property 26: 历史请求查询过滤**
    - **Validates: Requirements 6.4, 6.5, 6.6**

  - [x] 10.6 实现回放执行器
    - 实现 ReplayExecutor 结构体
    - 实现回放请求逻辑（ReplayRequest）
    - 实现响应对比器（ResponseComparator）
    - 实现差异计算（Token、延迟、内容相似度）
    - _Requirements: 6.7, 6.8_

  - [ ]* 10.7 编写回放功能单元测试
    - 测试回放执行逻辑
    - 测试响应对比计算
    - _Requirements: 6.7, 6.8_

- [x] 11. 监控与运维实现
  - [x] 11.1 实现 Prometheus 指标采集
    - 集成 Prometheus Go 客户端
    - 实现请求指标（总数、错误数、延迟分布）
    - 实现路由指标（故障切换次数、API Key 使用分布）
    - 实现熔断限流指标（熔断次数、限流次数）
    - 实现配额指标（Token 使用量、配额不足次数）
    - 实现系统指标（CPU、内存、Goroutine 数量）
    - _Requirements: 7.4, 7.5, 7.6_

  - [ ]* 11.2 编写监控指标属性测试
    - **Property 29: 监控指标累加准确性**
    - **Validates: Requirements 7.5, 7.6**

  - [x] 11.3 实现管理 API
    - 实现路由规则管理 API（创建、更新、删除、查询）
    - 实现配额管理 API（查询、调整）
    - 实现健康状态查询 API
    - 实现统计查询 API
    - _Requirements: 7.2, 7.3_

  - [ ]* 11.4 编写管理 API 单元测试
    - 测试路由规则 CRUD
    - 测试配额管理操作
    - _Requirements: 7.2, 7.3_

- [x] 12. 安全与认证实现
  - [x] 12.1 实现 API Key 认证
    - 实现认证中间件
    - 实现 Authorization 头解析
    - 实现 API Key 验证逻辑
    - _Requirements: 9.1, 9.2, 9.3_

  - [ ]* 12.2 编写认证属性测试
    - **Property 31: 认证失败错误响应**
    - **Validates: Requirements 9.2, 9.3**

  - [x] 12.3 实现 API Key 管理
    - 创建 api_keys 表结构
    - 实现 API Key CRUD 操作
    - 实现 API Key 与配额关联
    - 实现 API Key 使用统计
    - _Requirements: 9.4, 9.5, 9.6_

  - [ ]* 12.4 编写 API Key 管理单元测试
    - 测试 API Key CRUD
    - 测试配额隔离
    - _Requirements: 9.4, 9.5_

  - [x] 12.5 实现敏感配置加密
    - 实现配置加密/解密工具
    - 实现上游 API Key 加密存储
    - _Requirements: 9.7_

- [x] 13. Checkpoint - 验证安全与认证
  - 确保所有认证和安全测试通过，询问用户是否有问题

- [x] 14. 集成与端到端测试
  - [x] 14.1 实现端到端测试框架
    - 搭建测试环境（Redis、Kafka、PostgreSQL）
    - 实现测试辅助工具（Mock 上游服务）
    - _Requirements: 8.4_

  - [x] 14.2 编写完整请求流程测试
    - 测试非流式请求完整流程
    - 测试流式请求完整流程
    - 测试故障切换流程
    - 测试熔断和限流流程
    - _Requirements: 1.1, 1.2, 1.3, 2.5, 2.6, 2.7, 3.2, 3.7_

  - [x] 14.3 编写性能基准测试
    - 测试限流 P99 延迟 < 2ms
    - 测试 1000 并发 SSE 首 Token 延迟增量 < 15ms
    - 测试路由规则更新 < 1 秒生效
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 14.4 编写错误处理测试
    - 测试各类错误响应格式
    - 测试错误恢复机制
    - _Requirements: 1.6, 3.3, 5.6, 9.2, 9.3_

- [x] 15. 部署与文档
  - [x] 15.1 编写 Dockerfile 和 Docker Compose
    - 创建多阶段构建 Dockerfile
    - 创建 docker-compose.yml（包含所有依赖服务）
    - _Requirements: 8.5_

  - [x] 15.2 编写 Kubernetes 部署配置
    - 创建 Deployment、Service、ConfigMap、Secret
    - 创建 HorizontalPodAutoscaler
    - _Requirements: 8.5, 8.6_

  - [x] 15.3 编写 API 文档
    - 编写 OpenAPI/Swagger 规范
    - 编写 API 使用示例
    - _Requirements: 7.2, 7.3_

  - [x] 15.4 编写部署和运维文档
    - 编写部署指南
    - 编写配置说明
    - 编写监控和告警配置
    - 编写故障排查指南
    - _Requirements: 7.1, 7.4_

- [x] 16. Final Checkpoint - 完整性验证
  - 确保所有测试通过，所有文档完成，询问用户是否准备发布

## Notes

- 任务标记 `*` 的为可选测试任务，可根据项目进度决定是否实施
- 每个 Checkpoint 任务用于阶段性验证和用户确认
- 属性测试使用 gopter 库，最小迭代次数 100 次
- 单元测试目标覆盖率 ≥ 80%
- 性能测试需在接近生产环境的配置下进行
- 所有任务都引用了对应的需求编号，确保可追溯性
