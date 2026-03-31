package metrics

// Metrics 监控指标接口
type Metrics interface {
	// IncRequestTotal 增加请求总数
	IncRequestTotal(model string)

	// IncErrorTotal 增加错误总数
	IncErrorTotal(model string, errorType string)

	// ObserveLatency 记录延迟
	ObserveLatency(model string, latencyMs float64)

	// IncCircuitBreakerOpen 增加熔断次数
	IncCircuitBreakerOpen(key string)

	// IncRateLimitReject 增加限流拒绝次数
	IncRateLimitReject(dimension string)
}

// PrometheusMetrics Prometheus 指标实现
type PrometheusMetrics struct {
	// TODO: 实现 Prometheus 指标采集
}

// NewPrometheusMetrics 创建 Prometheus 指标
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{}
}

func (m *PrometheusMetrics) IncRequestTotal(model string) {
	// TODO: 实现
}

func (m *PrometheusMetrics) IncErrorTotal(model string, errorType string) {
	// TODO: 实现
}

func (m *PrometheusMetrics) ObserveLatency(model string, latencyMs float64) {
	// TODO: 实现
}

func (m *PrometheusMetrics) IncCircuitBreakerOpen(key string) {
	// TODO: 实现
}

func (m *PrometheusMetrics) IncRateLimitReject(dimension string) {
	// TODO: 实现
}
