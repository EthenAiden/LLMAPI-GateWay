package adapter

import (
	"fmt"
)

// AdapterFactory 协议适配器工厂
type AdapterFactory struct {
	adapters map[ProviderType]ProtocolAdapter
}

// NewAdapterFactory 创建适配器工厂
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		adapters: map[ProviderType]ProtocolAdapter{
			ProviderCursor:      NewCursorAdapter(),
			ProviderKiro:        NewKiroAdapter(),
			ProviderAntigravity: NewAntigravityAdapter(),
		},
	}
}

// GetAdapter 获取指定提供商的适配器
func (f *AdapterFactory) GetAdapter(provider ProviderType) (ProtocolAdapter, error) {
	adapter, ok := f.adapters[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
	return adapter, nil
}
