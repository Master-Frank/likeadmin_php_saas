package ctxutil

import (
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type Source string

const (
	SourcePlatform Source = "__saas__platform__"
	SourceTenant   Source = "__saas__tenant__"
	SourceUser     Source = "__saas__user__"
)

type AdminInfo map[string]any

type RequestMeta struct {
	Source       Source
	TenantID     uint
	TenantSN     string
	Tactics      int
	AdminInfo    AdminInfo
	AdminID      uint
	UserInfo     map[string]any
	UserID       uint
	App          string
	Controller   string
	Action       string
	NotNeedLogin bool
}

const key = "likeadmin.meta"

func Set(c *gin.Context, meta *RequestMeta) {
	c.Set(key, meta)
}

func Get(c *gin.Context) *RequestMeta {
	if c == nil {
		return &RequestMeta{}
	}
	v, ok := c.Get(key)
	if !ok {
		m := &RequestMeta{}
		Set(c, m)
		return m
	}
	return v.(*RequestMeta)
}

func ClientIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	rip := remoteIP(c)
	if fromTrustedProxy(rip) {
		if ip := strings.TrimSpace(c.GetHeader("X-Real-IP")); ip != "" {
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	if rip != "" {
		return rip
	}
	return c.RemoteIP()
}

func remoteIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := c.Request.RemoteAddr
	if ip, _, err := net.SplitHostPort(host); err == nil {
		return ip
	}
	return host
}

func fromTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range trustedProxyNets() {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func trustedProxyNets() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv("LIKEADMIN_TRUSTED_PROXIES"))
	if raw == "" {
		raw = "127.0.0.0/8,::1/128"
	}
	out := make([]*net.IPNet, 0, 4)
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if ip := net.ParseIP(p); ip != nil {
			if ip.To4() != nil {
				p += "/32"
			} else {
				p += "/128"
			}
		}
		_, n, err := net.ParseCIDR(p)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func Scheme(c *gin.Context) string {
	if c != nil && c.Request != nil && c.Request.TLS != nil {
		return "https"
	}
	if c != nil && fromTrustedProxy(remoteIP(c)) && strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return "https"
	}
	return "http"
}

func Domain(c *gin.Context) string {
	return Scheme(c) + "://" + Host(c)
}

func Host(c *gin.Context) string {
	host := ""
	if c != nil && c.Request != nil {
		host = c.Request.Host
		if fromTrustedProxy(remoteIP(c)) {
			if fwd := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); fwd != "" {
				host = fwd
			}
		}
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if len(host) > 253 {
		host = host[:253]
	}
	return host
}

func SubDomain(host string) string {
	if i := indexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if isIPv4(host) {
		return ""
	}
	parts := split(host, ".")
	if len(parts) < 2 {
		return ""
	}
	root := parts[len(parts)-2] + "." + parts[len(parts)-1]
	if host == root {
		return ""
	}
	return trimDotSuffix(host[:len(host)-len(root)])
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func split(s, sep string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			out = append(out, cur)
			cur = ""
			i += len(sep) - 1
			continue
		}
		cur += string(s[i])
	}
	out = append(out, cur)
	return out
}

func isIPv4(host string) bool {
	n := 0
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			n++
		} else if host[i] < '0' || host[i] > '9' {
			return false
		}
	}
	return n == 3
}

func trimDotSuffix(s string) string {
	for len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	return s
}
