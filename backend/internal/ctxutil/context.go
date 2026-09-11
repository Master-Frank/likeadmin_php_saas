package ctxutil

import (
	"net"
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
	// nginx overwrites X-Real-IP; ignore client-supplied X-Forwarded-For.
	if ip := strings.TrimSpace(c.GetHeader("X-Real-IP")); ip != "" {
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	if ip, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return ip
	}
	return c.RemoteIP()
}

func Domain(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	if fwd := c.GetHeader("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

func Host(c *gin.Context) string {
	host := c.Request.Host
	if fwd := c.GetHeader("X-Forwarded-Host"); fwd != "" {
		host = fwd
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
