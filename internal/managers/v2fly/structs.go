package v2fly

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/v2fly/v2ray-core/v5/infra/conf/serial"
	v4 "github.com/v2fly/v2ray-core/v5/infra/conf/v4"
	"k8s.io/apimachinery/pkg/util/json"
)

type V2rayLogConfig struct {
	// only in debug / info / warning / error / none
	// +kubebuilder:default=warning
	// +kubebuilder:validation:Enum=debug;info;warning;error;none
	Loglevel string `json:"loglevel,omitempty"`
	Access   string `json:"access,omitempty"` //	access log filepath
	Error    string `json:"error,omitempty"`  //	error log filepath
}

type V2rayInboundSettingsClientConfig struct {
	ID    string `json:"id,omitempty"`
	Level int    `json:"level,omitempty"`
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=false
	AlterId int    `json:"alterId,omitempty"`
	Email   string `json:"email,omitempty"`
}

type V2rayInboundSettingsFallbackConfig struct {
	Dest string `json:"dest,omitempty"`
	Path string `json:"path,omitempty"`
}

type V2rayInboundSettingsConfig struct {
	Address *string                            `json:"address,omitempty"` // only in `dokodemo-door`
	Clients []V2rayInboundSettingsClientConfig `json:"clients,omitempty"`
	//	must assign "none"
	// +kubebuilder:default=none
	Decryption string                               `json:"decryption,omitempty"`
	Fallbacks  []V2rayInboundSettingsFallbackConfig `json:"fallbacks,omitempty"`
}

type V2rayInboundStreamSettingsWsSettingsConfig struct {
	// +kubebuilder:default=false
	AcceptProxyProtocol bool   `json:"acceptProxyProtocol,omitempty"`
	Path                string `json:"path,omitempty"`
	// +kubebuilder:default=1024
	MaxEarlyData int `json:"maxEarlyData,omitempty"`
	// +kubebuilder:default=false
	UseBrowserForwarding bool   `json:"useBrowserForwarding,omitempty"`
	EarlyDataHeaderName  string `json:"earlyDataHeaderName,omitempty"`
}

type V2rayInboundStreamSettingsSockoptConfig struct {
	Mark        int  `json:"mark,omitempty"`
	TCPFastOpen bool `json:"tcpFastOpen,omitempty"`
	// +kubebuilder:default=4096
	TCPFastOpenQueueLength int `json:"tcpFastOpenQueueLength,omitempty"`
	// +kubebuilder:default=off
	Tproxy               string `json:"tproxy,omitempty"`
	TCPKeepAliveInterval int    `json:"tcpKeepAliveInterval,omitempty"`
}

type V2rayInboundStreamSettingsConfig struct {
	// +kubebuilder:default="ws"
	Network string `json:"network,omitempty"`
	// +kubebuilder:default="none"
	Security   string                                      `json:"security,omitempty"`
	WsSettings *V2rayInboundStreamSettingsWsSettingsConfig `json:"wsSettings,omitempty"`
	Sockopt    *V2rayInboundStreamSettingsSockoptConfig    `json:"sockopt,omitempty"`
}

type V2rayInboundConfig struct {
	Tag string `json:"tag,omitempty"`

	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	// +kubebuilder:validation:Minimum=1025
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=false
	//	default with random value
	Port int `json:"port,omitempty"`
	// +kubebuilder:validation:Enum=vless;vmess;trojan;dokodemo-door
	Protocol       string                            `json:"protocol,omitempty"`
	Settings       *V2rayInboundSettingsConfig       `json:"settings,omitempty"`
	StreamSettings *V2rayInboundStreamSettingsConfig `json:"streamSettings,omitempty"`
	// kubebuilder:validation:Pattern="^(\d{1,3}\.){3}\d{1,3}$"
	// +kubebuilder:default="127.0.0.1"
	Listen string `json:"listen,omitempty"`
}

type V2rayOutboundSettingsVnextUserConfig struct {
	Id         string `json:"id,omitempty"`
	Encryption string `json:"encryption,omitempty"` //	only vless
	AlterId    int    `json:"alterId,omitempty"`    //	only vmess
	Security   string `json:"security,omitempty"`   //	only vmess
	Level      int    `json:"level,omitempty"`
}

type V2rayOutboundSettingsVnextConfig struct {
	Address string                                 `json:"address,omitempty"`
	Port    int                                    `json:"port,omitempty"`
	Users   []V2rayOutboundSettingsVnextUserConfig `json:"users,omitempty"`
}

type V2rayOutboundSettingsConfig struct {
	//	freedom
	DomainStrategy string `json:"domainStrategy,omitempty"`
	Redirect       string `json:"redirect,omitempty"`
	UserLevel      int    `json:"userLevel,omitempty"`

	//	vless / vmess
	Vnext []V2rayOutboundSettingsVnextConfig `json:"vnext,omitempty"`

	//	loopback
	InboundTag string `json:"inboundTag,omitempty"`
}

type V2rayOutboundConfig struct {
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:validation:Enum=blackhole;freedom;dns;http;tls;vless;shadowsocks;vmess;socks;http2;loopback
	// +kubebuilder:default="freedom"
	Protocol string                       `json:"protocol,omitempty"`
	Settings *V2rayOutboundSettingsConfig `json:"settings,omitempty"`
}

type V2rayStatsConfig struct {
}

type V2rayPolicyLevelsConfig struct {
	Handshake         int  `json:"handshake,omitempty"`
	ConnIdle          int  `json:"connIdle,omitempty"`
	UplinkOnly        int  `json:"uplinkOnly,omitempty"`
	DownlinkOnly      int  `json:"downlinkOnly,omitempty"`
	StatsUserUplink   bool `json:"statsUserUplink,omitempty"`
	StatsUserDownlink bool `json:"statsUserDownlink,omitempty"`
	BufferSize        int  `json:"bufferSize,omitempty"`
}

type V2rayPolicySystemConfig struct {
	StatsInboundUplink    *bool `json:"statsInboundUplink,omitempty"`
	StatsInboundDownlink  *bool `json:"statsInboundDownlink,omitempty"`
	StatsOutboundUplink   *bool `json:"statsOutboundUplink,omitempty"`
	StatsOutboundDownlink *bool `json:"statsOutboundDownlink,omitempty"`
}

type V2rayPolicyConfig struct {
	Levels map[string]V2rayPolicyLevelsConfig `json:"levels,omitempty"`
	System *V2rayPolicySystemConfig           `json:"system,omitempty"`
}

type V2rayAPIConfig struct {
	Tag      string   `json:"tag,omitempty"`
	Services []string `json:"services,omitempty"`
}

type V2rayTransportTLSSettingsConfig struct{}

type V2rayTransportWsSettingsConfig struct {
	AcceptProxyProtocol  bool   `json:"acceptProxyProtocol,omitempty"`
	Path                 string `json:"path,omitempty"`
	MaxEarlyData         int    `json:"maxEarlyData,omitempty"`
	UseBrowserForwarding bool   `json:"useBrowserForwarding,omitempty"`
	EarlyDataHeaderName  string `json:"earlyDataHeaderName,omitempty"`
}

type V2rayTransportSockoptConfig struct {
	Mark                   int    `json:"mark,omitempty"`
	TCPFastOpen            bool   `json:"tcpFastOpen,omitempty"`
	TCPFastOpenQueueLength int    `json:"tcpFastOpenQueueLength,omitempty"`
	Tproxy                 string `json:"tproxy,omitempty"`
	TCPKeepAliveInterval   int    `json:"tcpKeepAliveInterval,omitempty"`
}

type V2rayTransportConfig struct {
	TLSSettings *V2rayTransportTLSSettingsConfig `json:"tlsSettings,omitempty"`
	WsSettings  *V2rayTransportWsSettingsConfig  `json:"wsSettings,omitempty"`
	Sockopt     *V2rayTransportSockoptConfig     `json:"sockopt,omitempty"`
}

type V2rayRoutingRulesSettingsRulesConfig struct {
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	Type        string   `json:"type,omitempty"`
}

type V2rayRoutingRulesSettingsConfig struct {
	Rules []V2rayRoutingRulesSettingsRulesConfig `json:"rules,omitempty"`
}

type V2rayRoutingRulesConfig struct {
	Settings *V2rayRoutingRulesSettingsConfig `json:"settings,omitempty"`
	Strategy string                           `json:"strategy,omitempty"`
}

type V2rayInstanceConfig struct {
	Log       V2rayLogConfig           `json:"log,omitempty"`
	Inbounds  []V2rayInboundConfig     `json:"inbounds,omitempty"`
	Outbounds []V2rayOutboundConfig    `json:"outbounds,omitempty"`
	Transport *V2rayTransportConfig    `json:"transport,omitempty"`
	Policy    *V2rayPolicyConfig       `json:"policy,omitempty"`
	API       *V2rayAPIConfig          `json:"api,omitempty"`
	Stats     *V2rayStatsConfig        `json:"stats,omitempty"`
	Routing   *V2rayRoutingRulesConfig `json:"routing,omitempty"`
}

func (this *V2rayInstanceConfig) DeepCopyInto(out *V2rayInstanceConfig) {
	*out = *this
	out.Log = this.Log
	if this.Inbounds != nil {
		in, out := &this.Inbounds, &out.Inbounds
		*out = make([]V2rayInboundConfig, len(*in))
		copy(*out, *in)
	}
	if this.Outbounds != nil {
		in, out := &this.Outbounds, &out.Outbounds
		*out = make([]V2rayOutboundConfig, len(*in))
		copy(*out, *in)
	}
	out.Transport = this.Transport
	out.Policy = this.Policy
	out.API = this.API
	out.Stats = this.Stats
	out.Routing = this.Routing
}

func (this *V2rayInstanceConfig) DeepCopy() *V2rayInstanceConfig {
	if this == nil {
		return nil
	}
	out := new(V2rayInstanceConfig)
	this.DeepCopyInto(out)
	return out
}

func (this *V2rayInstanceConfig) ToV2rayV4Config() (*v4.Config, error) {
	data, err := json.Marshal(this)
	if err != nil {
		return nil, fmt.Errorf("ToV2rayGrpcConfig marshal error: %w", err)
	}

	v4cfg, err := serial.DecodeJSONConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ToV2rayGrpcConfig parse error: %w", err)
	}

	return v4cfg, nil
}

//	v2rayN shared link format doc
//	https://github.com/2dust/v2rayN/wiki/%E5%88%86%E4%BA%AB%E9%93%BE%E6%8E%A5%E6%A0%BC%E5%BC%8F%E8%AF%B4%E6%98%8E(ver-2)

type V2rayNSharedLink map[string]any

func NewV2rayNSharedLinkFromV2rayInbound(cfg *V2rayInboundConfig) (V2rayNSharedLink, error) {
	var defaultDomain = "external.hdcjh.xyz"
	ret := make(map[string]any)

	switch cfg.Protocol {
	case "vmess":
		//
		ret["v"] = "2"
		ret["ps"] = cfg.Tag
		ret["add"] = defaultDomain
		ret["port"] = fmt.Sprintf("%d", cfg.Port)
		ret["id"] = cfg.Settings.Clients[0].ID
		ret["aid"] = fmt.Sprintf("%d", cfg.Settings.Clients[0].AlterId)
		ret["net"] = cfg.StreamSettings.Network
		ret["type"] = cfg.StreamSettings.Security
		//ret["host"] = defaultDomain
		ret["path"] = cfg.StreamSettings.WsSettings.Path
		ret["tls"] = "tls"
		ret["sni"] = defaultDomain
	default:
		return nil, fmt.Errorf("NewV2rayNSharedLinkFromV2rayInbound: unsupported protocol %s", cfg.Protocol)
	}

	return ret, nil
}

func (this V2rayNSharedLink) ToLink() string {
	prefix := "vmess://"
	var ret = bytes.NewBuffer(nil)
	if err := json.NewEncoder(ret).Encode(this); err != nil {
		panic(fmt.Errorf("V2rayNSharedLink.ToLink marshal failed: %+v", this))
	}
	return prefix + base64.StdEncoding.EncodeToString(ret.Bytes())
}

func (this V2rayNSharedLink) ToJson() string {
	var ret = bytes.NewBuffer(nil)
	if err := json.NewEncoder(ret).Encode(this); err != nil {
		panic(fmt.Errorf("V2rayNSharedLink.ToJson marshal failed: %+v", this))
	}
	return ret.String()
}

type T struct {
	Log struct {
		Loglevel string `json:"loglevel"`
	} `json:"log"`
	Inbounds []struct {
		Tag      string `json:"tag"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Settings struct {
			Clients []struct {
				Id    string `json:"id"`
				Level int    `json:"level"`
				Email string `json:"email"`
			} `json:"clients,omitempty"`
			Decryption string `json:"decryption,omitempty"`
			Fallbacks  []struct {
				Dest string `json:"dest"`
				Path string `json:"path"`
			} `json:"fallbacks,omitempty"`
			Address string `json:"address,omitempty"`
		} `json:"settings"`
		StreamSettings struct {
			Network    string `json:"network"`
			Security   string `json:"security"`
			WsSettings struct {
				AcceptProxyProtocol  bool   `json:"acceptProxyProtocol"`
				Path                 string `json:"path"`
				MaxEarlyData         int    `json:"maxEarlyData"`
				UseBrowserForwarding bool   `json:"useBrowserForwarding"`
				EarlyDataHeaderName  string `json:"earlyDataHeaderName"`
			} `json:"wsSettings"`
			Sockopt struct {
				Mark                   int    `json:"mark"`
				TcpFastOpen            bool   `json:"tcpFastOpen"`
				TcpFastOpenQueueLength int    `json:"tcpFastOpenQueueLength"`
				Tproxy                 string `json:"tproxy"`
				TcpKeepAliveInterval   int    `json:"tcpKeepAliveInterval"`
			} `json:"sockopt"`
		} `json:"streamSettings,omitempty"`
		Listen string `json:"listen,omitempty"`
	} `json:"inbounds"`
	Outbounds []struct {
		Tag      string `json:"tag"`
		Protocol string `json:"protocol"`
		Settings struct {
		} `json:"settings"`
	} `json:"outbounds"`
	Transport struct {
		TlsSettings struct {
		} `json:"tlsSettings"`
		WsSettings struct {
			AcceptProxyProtocol  bool   `json:"acceptProxyProtocol"`
			Path                 string `json:"path"`
			MaxEarlyData         int    `json:"maxEarlyData"`
			UseBrowserForwarding bool   `json:"useBrowserForwarding"`
			EarlyDataHeaderName  string `json:"earlyDataHeaderName"`
		} `json:"wsSettings"`
		Sockopt struct {
			Mark                   int    `json:"mark"`
			TcpFastOpen            bool   `json:"tcpFastOpen"`
			TcpFastOpenQueueLength int    `json:"tcpFastOpenQueueLength"`
			Tproxy                 string `json:"tproxy"`
			TcpKeepAliveInterval   int    `json:"tcpKeepAliveInterval"`
		} `json:"sockopt"`
	} `json:"transport"`
	Policy struct {
		Levels struct {
			Field1 struct {
				Handshake         int  `json:"handshake"`
				ConnIdle          int  `json:"connIdle"`
				UplinkOnly        int  `json:"uplinkOnly"`
				DownlinkOnly      int  `json:"downlinkOnly"`
				StatsUserUplink   bool `json:"statsUserUplink"`
				StatsUserDownlink bool `json:"statsUserDownlink"`
				BufferSize        int  `json:"bufferSize"`
			} `json:"0"`
		} `json:"levels"`
		System struct {
			StatsInboundUplink    bool `json:"statsInboundUplink"`
			StatsInboundDownlink  bool `json:"statsInboundDownlink"`
			StatsOutboundUplink   bool `json:"statsOutboundUplink"`
			StatsOutboundDownlink bool `json:"statsOutboundDownlink"`
		} `json:"system"`
	} `json:"policy"`
	Api struct {
		Tag      string   `json:"tag"`
		Services []string `json:"services"`
	} `json:"api"`
	Stats struct {
	} `json:"stats"`
	Routing struct {
		Settings struct {
			Rules []struct {
				InboundTag  []string `json:"inboundTag"`
				OutboundTag string   `json:"outboundTag"`
				Type        string   `json:"type"`
			} `json:"rules"`
		} `json:"settings"`
		Strategy string `json:"strategy"`
	} `json:"routing"`
}
