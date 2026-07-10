package ai2web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuilderAndValidate(t *testing.T) {
	m := ForSite("Example Store", "https://store.example.com", "ecommerce").
		Capability("content", nil).
		Capability("commerce", map[string]any{"endpoint": "/ai2w/products", "checkout": true}).
		Capability("search", map[string]any{"endpoint": "/ai2w/search"}).
		Transports(map[string]any{
			"mcp":  map[string]any{"enabled": true, "endpoint": "/ai2w/mcp"},
			"rest": map[string]any{"enabled": true},
		}).
		Auth(map[string]any{"methods": []any{"none", "oauth2"}, "oauth2": map[string]any{"pkce": true}}).
		Consent(map[string]any{"requires_user_approval_for": []any{"purchase"}}).
		Contact(map[string]any{"support": "help@store.example.com"}).
		Identity(map[string]any{"legal_name": "Example Store Ltd"}).
		Build()

	r := Validate(m)
	if !r.Valid {
		t.Fatalf("expected valid, errors=%v", r.Errors)
	}
	if r.Score < 90 {
		t.Fatalf("score %d < 90", r.Score)
	}
	if r.Tier != "Standard" && r.Tier != "Enterprise" {
		t.Fatalf("tier %s", r.Tier)
	}
}

func TestNegotiate(t *testing.T) {
	m := ForSite("S", "https://s.example", "ecommerce").
		Capability("content", nil).
		Capability("commerce", map[string]any{"endpoint": "/ai2w/products"}).
		Transports(map[string]any{"mcp": map[string]any{"enabled": true}, "rest": map[string]any{"enabled": true}}).
		Auth(map[string]any{"methods": []any{"none", "oauth2"}}).
		Build()

	n := Negotiate(m, map[string]any{
		"transports":   []any{"mcp", "rest"},
		"capabilities": []any{"content", "commerce", "flying"},
		"auth":         []any{"oauth2"},
	})
	if n.Negotiated.Transport == nil || *n.Negotiated.Transport != "mcp" {
		t.Fatalf("transport = %v", n.Negotiated.Transport)
	}
	if len(n.Negotiated.Capabilities) != 2 {
		t.Fatalf("capabilities = %v", n.Negotiated.Capabilities)
	}
	if len(n.Unsupported) != 1 || n.Unsupported[0] != "flying" {
		t.Fatalf("unsupported = %v", n.Unsupported)
	}
	if n.Negotiated.Auth == nil || *n.Negotiated.Auth != "oauth2" {
		t.Fatalf("auth = %v", n.Negotiated.Auth)
	}
}

func TestServer(t *testing.T) {
	m := ForSite("S", "https://s.example", "content").Capability("content", nil).Build()
	if r := Handle(ServerOptions{Manifest: m}, "GET", "/ai2w", nil, ""); r.Status != 200 {
		t.Fatalf("manifest status %d", r.Status)
	}
	if r := Handle(ServerOptions{Manifest: m}, "POST", "/ai2w", nil, ""); r.Status != 405 {
		t.Fatalf("expected 405, got %d", r.Status)
	}
	wk := Handle(ServerOptions{Manifest: m}, "GET", "/.well-known/ai2w", nil, "https://s.example")
	if b, _ := wk.Body.(map[string]any); b["ai2w"] != "https://s.example/ai2w" {
		t.Fatalf("well-known pointer = %v", wk.Body)
	}
}

func TestSafety(t *testing.T) {
	cases := map[string]bool{
		"https://store.example.com":      true,
		"http://169.254.169.254/latest":  false,
		"http://localhost:8080":          false,
		"https://10.0.0.5/x":             false,
		"file:///etc/passwd":             false,
		"https://192.168.1.1":            false,
	}
	for u, exp := range cases {
		if got := IsSafePublicURL(u); got != exp {
			t.Errorf("IsSafePublicURL(%s) = %v, want %v", u, got, exp)
		}
	}
}

func TestConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string         `json:"name"`
		Manifest map[string]any `json:"manifest"`
		Expect   map[string]any `json:"expect"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		r := Validate(c.Manifest)
		e := c.Expect
		if v, ok := e["valid"].(bool); ok && r.Valid != v {
			t.Errorf("%s: valid=%v want %v", c.Name, r.Valid, v)
		}
		if tier, ok := e["tier"].(string); ok && r.Tier != tier {
			t.Errorf("%s: tier=%s want %s", c.Name, r.Tier, tier)
		}
		if ms, ok := e["minScore"].(float64); ok && float64(r.Score) < ms {
			t.Errorf("%s: score=%d < %v", c.Name, r.Score, ms)
		}
		if ec, ok := e["errorsContain"].(string); ok {
			found := false
			for _, x := range r.Errors {
				if strings.Contains(x, ec) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: errors missing %q: %v", c.Name, ec, r.Errors)
			}
		}
		if warns, ok := e["warns"].([]any); ok {
			for _, w := range warns {
				label, _ := w.(string)
				var chk *Check
				for i := range r.Checks {
					if r.Checks[i].Label == label {
						chk = &r.Checks[i]
					}
				}
				if chk == nil || chk.OK {
					t.Errorf("%s: expected warning on %q", c.Name, label)
				}
			}
		}
	}
}
