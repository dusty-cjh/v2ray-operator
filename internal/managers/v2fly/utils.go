package v2fly

import (
	"encoding/base64"
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"strings"
)

func NewV2rayNSharedLinkFromLink(link string) (V2rayNSharedLink, error) {
	//	validate and remove the `vless://` prefix
	link, ok := strings.CutPrefix(link, "vmess://")
	if !ok {
		return nil, fmt.Errorf("NewV2rayNSharedLinkFromLink: invalid link format")
	}

	var ret V2rayNSharedLink = make(map[string]any)
	data, err := base64.StdEncoding.DecodeString(link)
	if err != nil {
		return nil, fmt.Errorf("NewV2rayNSharedLinkFromLink: decode link failed: %w", err)
	}
	if err := json.Unmarshal(data, &ret); err != nil {
		return nil, fmt.Errorf("NewV2rayNSharedLinkFromLink: unmarshal link failed: %w", err)
	}
	return ret, nil
}

func ToV2rayNSubscriptionLinkContent(linkList []V2rayNSharedLink) string {
	ret := strings.Builder{}
	for _, link := range linkList {
		ret.WriteString(link.ToLink())
		ret.WriteString("\n")
	}
	//	base64 encode
	return base64.StdEncoding.EncodeToString([]byte(ret.String()))
}

func FromV2rayNSubscriptionLinkContent(content string) ([]V2rayNSharedLink, error) {
	//	decode base64
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("FromV2rayNSubscriptionLinkContent: decode content failed: %w", err)
	}
	//	split by \n
	lines := strings.Split(string(data), "\n")
	//	decode each line
	ret := make([]V2rayNSharedLink, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		link, err := NewV2rayNSharedLinkFromLink(line)
		if err != nil {
			return nil, fmt.Errorf("FromV2rayNSubscriptionLinkContent: decode line failed: %w", err)
		}
		ret = append(ret, link)
	}

	return ret, nil
}
