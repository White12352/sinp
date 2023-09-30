package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	O "github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

type SMap struct {
	sync.RWMutex
	Map map[string]adapter.Outbound
}

type SubscriptionInfo struct {
	upload   uint64
	download uint64
	total    uint64
	expire   uint64
}

type myProviderAdapter struct {
	subscriptionInfo SubscriptionInfo
	logger           log.ContextLogger
	tag              string
	path             string
	healthCheckUrl   string
	providerType     string
	updateTime       time.Time
	outbounds        []adapter.Outbound
	outboundByTag    SMap
	isUpdating       bool
}

func (a *myProviderAdapter) Tag() string {
	return a.tag
}

func (a *myProviderAdapter) Path() string {
	return a.path
}

func (a *myProviderAdapter) Type() string {
	return a.providerType
}

func (a *myProviderAdapter) HealthCheckUrl() string {
	return a.healthCheckUrl
}

func (a *myProviderAdapter) UpdateTime() time.Time {
	return a.updateTime
}

func (a *myProviderAdapter) IsUpdating() bool {
	return a.isUpdating
}

func (a *myProviderAdapter) Outbound(tag string) (adapter.Outbound, bool) {
	a.outboundByTag.RLock()
	outbound, loaded := a.outboundByTag.Map[tag]
	a.outboundByTag.RUnlock()
	return outbound, loaded
}

func (a *myProviderAdapter) Outbounds() []adapter.Outbound {
	return a.outbounds
}

func GetFirstLine(content string) (string, string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 1 {
		return lines[0], ""
	}
	others := strings.Join(lines[1:], "\n")
	return lines[0], others
}

func (a *myProviderAdapter) SubscriptionInfo() map[string]uint64 {
	info := make(map[string]uint64)
	info["Upload"] = a.subscriptionInfo.upload
	info["Download"] = a.subscriptionInfo.download
	info["Total"] = a.subscriptionInfo.total
	info["Expire"] = a.subscriptionInfo.expire
	return info
}

func (a *myProviderAdapter) ParseSubInfo(infoString string) bool {
	reg := regexp.MustCompile("upload=(\\d*);[ \t]*download=(\\d*);[ \t]*total=(\\d*);[ \t]*expire=(\\d*)")
	result := reg.FindStringSubmatch(infoString)
	if len(result) > 0 {
		upload, _ := strconv.Atoi(result[1:][0])
		download, _ := strconv.Atoi(result[1:][1])
		total, _ := strconv.Atoi(result[1:][2])
		expire, _ := strconv.Atoi(result[1:][3])
		a.subscriptionInfo.upload = uint64(upload)
		a.subscriptionInfo.download = uint64(download)
		a.subscriptionInfo.total = uint64(total)
		a.subscriptionInfo.expire = uint64(expire)
		return true
	}
	return false
}

func (a *myProviderAdapter) CreateOutboundFromContent(ctx context.Context, router adapter.Router, outbounds []option.Outbound) error {
	for _, outbound := range outbounds {
		otype := outbound.Type
		tag := outbound.Tag
		switch otype {
		case C.TypeDirect, C.TypeBlock, C.TypeDNS, C.TypeSelector, C.TypeURLTest:
			continue
		default:
			out, err := O.New(ctx, router, a.logger, tag, outbound)
			if err != nil {
				E.New("invalid outbound")
				continue
			}
			a.outboundByTag.Map[tag] = out
			a.outbounds = append(a.outbounds, out)
		}
	}
	return nil
}

func TrimBlank(str string) string {
	str = strings.Trim(str, " ")
	str = strings.Trim(str, "\t")
	return str
}

func GetTrimedFile(path string) []byte {
	content, _ := os.ReadFile(path)
	return []byte(TrimBlank(string(content)))
}

func (p *myProviderAdapter) GetContentFromFile(router adapter.Router) string {
	content := ""
	updateTime := time.Unix(int64(0), int64(0))
	path := p.path
	fileInfo, err := os.Stat(path)
	if !os.IsNotExist(err) {
		updateTime = fileInfo.ModTime()
		contentRaw := GetTrimedFile(path)
		content = string(contentRaw)
		firstLine, others := GetFirstLine(content)
		if p.ParseSubInfo(firstLine) {
			content = others
		}
	}
	p.updateTime = updateTime
	return content
}

func DecodeBase64Safe(content string) string {
	reg := regexp.MustCompile("^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{4}|[A-Za-z0-9+/]{3}=?|[A-Za-z0-9+/]{2}(==)?)$")
	if len(reg.FindStringSubmatch(content)) > 0 {
		decode, err := base64.StdEncoding.DecodeString(content)
		if err == nil {
			return string(decode)
		}
	}
	return content
}

func ParseContent(contentRaw string) []byte {
	content := DecodeBase64Safe(contentRaw)
	return []byte(content)
}

func (p *myProviderAdapter) ParseOutbounds(ctx context.Context, router adapter.Router, content []byte) error {
	if len(content) == 0 {
		return nil
	}
	var options option.Options
	err := options.UnmarshalJSON(content)
	if err != nil {
		return E.Cause(err, "decode config at ", p.path)
	}
	err = p.CreateOutboundFromContent(ctx, router, options.Outbounds)
	if err != nil {
		return err
	}
	return nil
}

func (p *myProviderAdapter) BackupProvider() ([]adapter.Outbound, map[string]adapter.Outbound, SubscriptionInfo) {
	outboundsBackup := []adapter.Outbound{}
	outboundByTagBackup := make(map[string]adapter.Outbound)
	outboundsBackup = append(outboundsBackup, p.outbounds...)
	for tag, out := range p.outboundByTag.Map {
		outboundByTagBackup[tag] = out
	}
	subscriptionInfoBackup := SubscriptionInfo{
		upload:   p.subscriptionInfo.upload,
		download: p.subscriptionInfo.download,
		total:    p.subscriptionInfo.total,
		expire:   p.subscriptionInfo.expire,
	}
	p.outbounds = []adapter.Outbound{}
	p.outboundByTag.Map = make(map[string]adapter.Outbound)
	p.subscriptionInfo.upload = uint64(0)
	p.subscriptionInfo.download = uint64(0)
	p.subscriptionInfo.total = uint64(0)
	p.subscriptionInfo.expire = uint64(0)
	return outboundsBackup, outboundByTagBackup, subscriptionInfoBackup
}

func (p *myProviderAdapter) RevertProvider(outboundsBackup []adapter.Outbound, outboundByTagBackup map[string]adapter.Outbound, subscriptionInfoBackup SubscriptionInfo) {
	for _, out := range p.outbounds {
		common.Close(out)
	}
	p.outbounds = outboundsBackup
	p.outboundByTag.Map = outboundByTagBackup
	p.subscriptionInfo = subscriptionInfoBackup
}

func (p *myProviderAdapter) LockOutboundByTag() {
	p.outboundByTag.RLock()
}

func (p *myProviderAdapter) UnlockOutboundByTag() {
	p.outboundByTag.RUnlock()
}

func (p *myProviderAdapter) UpdateOutboundByTag() {
	p.outboundByTag.Map = make(map[string]adapter.Outbound)
	for _, out := range p.outbounds {
		tag := out.Tag()
		p.outboundByTag.Map[tag] = out
	}
}

func (p *myProviderAdapter) StartOutbounds(router adapter.Router) error {
	pTag := p.Tag()
	outboundTag := make(map[string]bool)
	for _, out := range router.Outbounds() {
		outboundTag[out.Tag()] = true
	}
	for _, p := range router.OutboundProviders() {
		if p.Tag() == pTag {
			continue
		}
		for _, out := range p.Outbounds() {
			outboundTag[out.Tag()] = true
		}
	}
	for i, out := range p.Outbounds() {
		var tag string
		if out.Tag() == "" {
			tag = fmt.Sprint("[", pTag, "]", F.ToString(i))
		} else {
			tag = out.Tag()
		}
		if _, exists := outboundTag[tag]; exists {
			i := 1
			for {
				tTag := fmt.Sprint(tag, "[", i, "]")
				if _, exists := outboundTag[tTag]; exists {
					i++
					continue
				}
				tag = tTag
				break
			}
			out.SetTag(tag)
		}
		outboundTag[tag] = true
		if starter, isStarter := out.(common.Starter); isStarter {
			p.logger.Trace("initializing outbound provider[", pTag, "]", " outbound/", out.Type(), "[", tag, "]")
			err := starter.Start()
			if err != nil {
				return E.Cause(err, "initialize outbound provider[", pTag, "]", " outbound/", out.Type(), "[", tag, "]")
			}
		}
	}
	p.UpdateOutboundByTag()
	return nil
}

func (p *myProviderAdapter) UpdateGroups(router adapter.Router) error {
	for _, outbound := range router.Outbounds() {
		if group, ok := outbound.(adapter.OutboundGroup); ok {
			err := group.UpdateOutbounds(p.tag)
			if err != nil {
				return E.Cause(err, "update provider ", p.tag, " failed")
			}
		}
	}
	return nil
}

func (p *myProviderAdapter) RunFuncsWithRevert(funcArray ...func() error) error {
	for _, funcToRun := range funcArray {
		err := funcToRun()
		if err != nil {
			return err
		}
	}
	return nil
}
