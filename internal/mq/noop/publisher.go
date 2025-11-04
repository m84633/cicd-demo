package noop

import (
	"context"
	"partivo_tickets/internal/mq"
)

// Publisher 是一個空操作的發布者，它實作了 mq.Publisher 介面但什麼也不做。
type Publisher struct{}

// NewPublisher 建立一個新的 NoOp Publisher。
func NewPublisher() *Publisher {
	return &Publisher{}
}

// Publish 什麼也不做，直接返回 nil。
func (p *Publisher) Publish(ctx context.Context, topic string, body []byte) error {
	return nil
}

// Close 什麼也不做。
func (p *Publisher) Close() {
	// No-op, 不需要關閉任何連線
}

// 確保 Publisher 實作了 mq.Publisher 介面
var _ mq.Publisher = (*Publisher)(nil)
