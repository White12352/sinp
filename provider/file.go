package provider

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.OutboundProvider = (*FileProvider)(nil)

type FileProvider struct {
	myProviderAdapter
}

func (p *FileProvider) ParseProvider(ctx context.Context, router adapter.Router) error {
	content := ParseContent(p.GetContentFromFile(router))
	return p.ParseOutbounds(ctx, router, content)
}

func (p *FileProvider) UpdateProvider(ctx context.Context, router adapter.Router) error {
	p.LockOutboundByTag()
	p.isUpdating = true
	outboundsBackup, outboundByTagBackup, subscriptionInfoBackup := p.BackupProvider()
	err := p.RunFuncsWithRevert(
		func() error { return p.ParseProvider(ctx, router) },
		func() error { return p.StartOutbounds(router) },
		func() error { return p.UpdateGroups(router) },
	)
	if err != nil {
		p.RevertProvider(outboundsBackup, outboundByTagBackup, subscriptionInfoBackup)
	}
	p.isUpdating = false
	p.UnlockOutboundByTag()
	return nil
}

func NewFileProvider(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.OutboundProvider) (*FileProvider, error) {
	provider := &FileProvider{
		myProviderAdapter: myProviderAdapter{
			logger:         logger,
			tag:            options.Tag,
			path:           options.Path,
			healthCheckUrl: options.HealthCheckUrl,
			providerType:   C.TypeFileProvider,
			updateTime:     time.Unix(int64(0), int64(0)),
			subscriptionInfo: SubscriptionInfo{
				upload:   0,
				download: 0,
				total:    0,
				expire:   0,
			},
			outbounds: []adapter.Outbound{},
			outboundByTag: SMap{
				Map: make(map[string]adapter.Outbound),
			},
			isUpdating: false,
		},
	}
	err := provider.ParseProvider(ctx, router)
	if err != nil {
		return nil, err
	}
	return provider, nil
}
