package v2fly

import (
	"strconv"
	"strings"
	"sync"
)

type V2rayConfig struct {
	Host string `json:"host" config:"host"`
	Port int    `json:"port" config:"port"`
}

func (this *V2rayConfig) ServerAddr() string {
	ret := strings.Builder{}
	ret.WriteString(this.Host)
	ret.WriteString(":")
	ret.WriteString(strconv.Itoa(this.Port))
	return ret.String()
}

var _v2ray_config_once sync.Once = sync.Once{}
var _v2ray_config *V2rayConfig
