// Package ai2web is the Go reference implementation of the AI2Web (ai2w) protocol.
//
// Describe your website once. AI2Web makes it understandable to every AI.
package ai2web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Manifest is a plain map so it round-trips with arbitrary JSON. Use Builder to construct one.
type Manifest = map[string]any

// ---- internal helpers (shared by validator/negotiator/server) ----

func toMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}

// has reports whether a capability value is enabled (true, or an object with enabled:true).
func has(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if m, ok := v.(map[string]any); ok {
		return m["enabled"] == true
	}
	return false
}

func boolInMap(v any, key string) bool {
	return toMap(v)[key] == true
}

func containsStr(s []any, target string) bool {
	for _, v := range s {
		if toStr(v) == target {
			return true
		}
	}
	return false
}

// ---- Builder: the fluent "describe your website once" surface ----

type Builder struct{ m Manifest }

// New starts a builder for a site.
func New(site map[string]any) *Builder {
	return &Builder{m: Manifest{"protocol": "ai2w", "version": "0.1", "site": site, "capabilities": map[string]any{}}}
}

// ForSite is a convenience over New.
func ForSite(name, url, typ string) *Builder {
	return New(map[string]any{"name": name, "url": url, "type": typ})
}

func (b *Builder) Capability(name string, value any) *Builder {
	if value == nil {
		value = true
	}
	if obj, ok := value.(map[string]any); ok {
		merged := map[string]any{"enabled": true}
		for k, v := range obj {
			merged[k] = v
		}
		value = merged
	}
	b.m["capabilities"].(map[string]any)[name] = value
	return b
}

func (b *Builder) Transports(t map[string]any) *Builder {
	tr, _ := b.m["transports"].(map[string]any)
	if tr == nil {
		tr = map[string]any{}
	}
	for k, v := range t {
		tr[k] = v
	}
	b.m["transports"] = tr
	return b
}

func (b *Builder) Auth(a map[string]any) *Builder    { b.m["auth"] = a; return b }
func (b *Builder) Consent(c map[string]any) *Builder { b.m["consent"] = c; return b }
func (b *Builder) Identity(i map[string]any) *Builder { b.m["identity"] = i; return b }
func (b *Builder) Contact(c map[string]any) *Builder { b.m["contact"] = c; return b }

func (b *Builder) Action(a map[string]any) *Builder {
	acts, _ := b.m["actions"].([]any)
	b.m["actions"] = append(acts, a)
	return b.Capability("actions", map[string]any{"endpoint": "/ai2w/actions"})
}

func (b *Builder) Events(e map[string]any) *Builder {
	b.m["events"] = e
	ep := "/ai2w/events"
	if s, ok := e["endpoint"].(string); ok {
		ep = s
	}
	return b.Capability("events", map[string]any{"endpoint": ep})
}

func (b *Builder) AgentService(s map[string]any) *Builder { b.m["agent_service"] = s; return b }

func (b *Builder) Build() Manifest { return b.m }

func (b *Builder) ToJSON() (string, error) {
	out, err := json.MarshalIndent(b.m, "", "  ")
	return string(out), err
}

// ---- Safety: SSRF guard (parity with @ai2web/core safety) ----

// IsSafePublicURL reports whether raw is a safe public http(s) target (not loopback,
// private, link-local, cloud-metadata, or CGNAT). Literal host/IP check; not DNS-rebind safe.
func IsSafePublicURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return false
		}
		if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 { // CGNAT
			return false
		}
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	return true
}

// AssertSafePublicURL returns an error if raw is not a safe public target.
func AssertSafePublicURL(raw string) error {
	if !IsSafePublicURL(raw) {
		return fmt.Errorf("ai2w: refusing to fetch non-public or unsafe URL: %s", raw)
	}
	return nil
}

// SameOrigin reports whether a and b share scheme+host(+port).
func SameOrigin(a, b string) bool {
	ua, ea := url.Parse(a)
	ub, eb := url.Parse(b)
	if ea != nil || eb != nil {
		return false
	}
	return ua.Scheme == ub.Scheme && ua.Host == ub.Host
}
