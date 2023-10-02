package adapter

import (
	"context"
	"time"
)

type OutboundProvider interface {
	Tag() string
	Path() string
	Type() string
	HealthCheckUrl() string
	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
	UpdateTime() time.Time
	SubscriptionInfo() map[string]uint64
	UpdateProvider(ctx context.Context, router Router) error
	UpdateOutboundByTag()
	LockOutboundByTag()
	UnlockOutboundByTag()
	IsUpdating() bool
}
