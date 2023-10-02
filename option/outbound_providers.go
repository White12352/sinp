package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type _OutboundProvider struct {
	Type           string                      `json:"type"`
	Path           string                      `json:"path"`
	Tag            string                      `json:"tag,omitempty"`
	HealthCheckUrl string                      `json:"healthcheck_url,omitempty"`
	HTTPOptions    HTTPOutboundProviderOptions `json:"-"`
}

type OutboundProvider _OutboundProvider

type HTTPOutboundProviderOptions struct {
	Url       string `json:"download_url"`
	UserAgent string `json:"download_ua,omitempty"`
	Detour    string `json:"download_detour,omitempty"`
}

func (h OutboundProvider) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.TypeFileProvider:
		v = nil
	case C.TypeHTTPProvider:
		v = h.HTTPOptions
	default:
		return nil, E.New("unknown provider type: ", h.Type)
	}
	return MarshallObjects((_OutboundProvider)(h), v)
}

func (h *OutboundProvider) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_OutboundProvider)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.TypeFileProvider:
		v = nil
	case C.TypeHTTPProvider:
		v = &h.HTTPOptions
	default:
		return E.New("unknown provider type: ", h.Type)
	}
	err = UnmarshallExcluded(bytes, (*_OutboundProvider)(h), v)
	if err != nil {
		return E.Cause(err, "provider options")
	}
	return nil
}
