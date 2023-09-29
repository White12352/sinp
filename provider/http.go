package provider

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

var _ adapter.OutboundProvider = (*HTTPProvider)(nil)

type HTTPProvider struct {
	myProviderAdapter
	url    string
	ua     string
	detour string
	start  bool
}

func (p *HTTPProvider) Start() bool {
	return p.start
}

func (p *HTTPProvider) SetStart() {
	p.start = true
}

func (p *HTTPProvider) FetchHTTP(httpClient *http.Client, parsedURL *url.URL) (string, string, error) {
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Add("User-Agent", p.ua)
	response, err := httpClient.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	contentRaw, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", err
	}
	if len(contentRaw) == 0 {
		return "", "", E.New("empty response")
	}
	content := string(contentRaw)
	subInfo := response.Header.Get("subscription-userinfo")
	if subInfo != "" {
		subInfo = "# " + subInfo + ";"
	}
	return content, subInfo, nil
}

func (p *HTTPProvider) FetchContent(router adapter.Router) (string, string, error) {
	detour := router.DefaultOutboundForConnection()
	if p.detour != "" {
		if outbound, ok := router.Outbound(p.detour); ok {
			detour = outbound
		}
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: true,
		},
	}
	defer httpClient.CloseIdleConnections()
	parsedURL, err := url.Parse(p.url)
	if err != nil {
		return "", "", err
	}
	switch parsedURL.Scheme {
	case "":
		parsedURL.Scheme = "http"
		fallthrough
	case "http", "https":
		content, subInfo, err := p.FetchHTTP(httpClient, parsedURL)
		if err != nil {
			return "", "", err
		}
		return content, subInfo, nil
	default:
		return "", "", E.New("invalid url scheme")
	}
}

func (p *HTTPProvider) ContentFromHTTP(router adapter.Router) string {
	contentRaw, subInfo, err := p.FetchContent(router)
	if err != nil {
		E.Cause(err, "fetch provider ", p.tag, " failed")
		return ""
	}
	path := p.path
	if ok := p.ParseSubInfo(subInfo); ok {
		contentRaw = subInfo + "\n" + contentRaw
	} else {
		firstLine, _ := GetFirstLine(contentRaw)
		p.ParseSubInfo(firstLine)
	}
	p.updateTime = time.Now()
	content := ParseContent(contentRaw)
	os.WriteFile(path, []byte(content), 0o666)
	return content
}

func (p *HTTPProvider) GetContent(router adapter.Router) string {
	if !p.start {
		p.start = true
		return p.GetContentFromFile(router)
	}
	return p.ContentFromHTTP(router)
}

func (p *HTTPProvider) ParseProvider(ctx context.Context, router adapter.Router) error {
	content := p.GetContent(router)
	return p.ParseOutbounds(ctx, router, content)
}

func (p *HTTPProvider) UpdateProvider(ctx context.Context, router adapter.Router) error {
	p.isUpdating = true
	p.outboundByTag.RLock()
	outboundsBackup, outboundByTagBackup, subscriptionInfoBackup := p.BackupProvider()
	err := p.RunFuncsWithRevert(
		func() error { return p.ParseProvider(ctx, router) },
		func() error { return p.StartOutbounds(router) },
		func() error { return p.UpdateGroups(router) },
	)
	if err != nil {
		p.RevertProvider(outboundsBackup, outboundByTagBackup, subscriptionInfoBackup)
	}
	p.outboundByTag.RUnlock()
	p.isUpdating = false
	return nil
}

func NewHTTPProvider(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.OutboundProvider) (*HTTPProvider, error) {
	httpOptions := options.HTTPOptions
	url := httpOptions.Url
	ua := httpOptions.UserAgent
	if url == "" {
		return nil, E.New("provider download url missing")
	}
	if ua == "" {
		ua = "sing-box"
	}
	provider := &HTTPProvider{
		myProviderAdapter: myProviderAdapter{
			logger:         logger,
			tag:            options.Tag,
			path:           options.Path,
			healthCheckUrl: options.HealthCheckUrl,
			providerType:   C.TypeHTTPProvider,
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
		url:    httpOptions.Url,
		ua:     ua,
		detour: httpOptions.Detour,
		start:  false,
	}
	err := provider.ParseProvider(ctx, router)
	if err != nil {
		return nil, err
	}
	return provider, nil
}
