package ai2web

import (
	"encoding/json"
	"os"
	"slices"
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

func TestSchemaAndInputValidation(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"order_id": map[string]any{"type": "string"}, "qty": map[string]any{"type": "integer"}},
		"required":   []any{"order_id"},
	}
	if ok, _ := ValidateSchema(map[string]any{"order_id": "A1", "qty": 2.0}, schema, "input"); !ok {
		t.Error("valid input should pass")
	}
	if ok, _ := ValidateSchema(map[string]any{"qty": 2.0}, schema, "input"); ok {
		t.Error("missing required should fail")
	}
	if ok, _ := ValidateSchema(map[string]any{"order_id": 5.0}, schema, "input"); ok {
		t.Error("wrong type should fail")
	}
	if ok, _ := ValidateSchema(map[string]any{"order_id": "A1", "qty": 1.5}, schema, "input"); ok {
		t.Error("non-integer should fail")
	}
	if ok, _ := ValidateSchema(map[string]any{"anything": 1}, map[string]any{}, "input"); !ok {
		t.Error("empty schema should accept anything")
	}

	man := Manifest{
		"protocol": "ai2w",
		"actions": []any{map[string]any{
			"name":         "track_order",
			"endpoint":     "/ai2w/actions/track-order",
			"input_schema": schema,
		}},
	}
	acts := map[string]Handler{"track_order": func(b any) any { return map[string]any{"ok": true} }}

	ok := Handle(ServerOptions{Manifest: man, Actions: acts}, "POST", "/ai2w/actions/track-order", map[string]any{"order_id": "A1"}, "")
	if ok.Status != 200 {
		t.Errorf("valid body: status %d", ok.Status)
	}
	bad := Handle(ServerOptions{Manifest: man, Actions: acts}, "POST", "/ai2w/actions/track-order", map[string]any{}, "")
	if bad.Status != 400 {
		t.Errorf("missing required: expected 400, got %d", bad.Status)
	}
	if b, _ := bad.Body.(map[string]any); b != nil {
		if e, _ := b["error"].(map[string]any); e == nil || e["code"] != "invalid_request" {
			t.Errorf("expected invalid_request error, got %v", bad.Body)
		}
	}
	off := false
	skip := Handle(ServerOptions{Manifest: man, Actions: acts, ValidateInput: &off}, "POST", "/ai2w/actions/track-order", map[string]any{}, "")
	if skip.Status != 200 {
		t.Errorf("validate-input opt-out: expected 200, got %d", skip.Status)
	}
}

func TestV02AndExport(t *testing.T) {
	m := ForSite("Example Bistro", "https://bistro.example", "restaurant").
		Capability("content", nil).
		Capability("commerce", map[string]any{"endpoint": "/ai2w/products"}).
		Capability("search", map[string]any{"endpoint": "/ai2w/search"}).
		Action(map[string]any{
			"name": "book_table", "description": "Reserve a table.", "method": "POST",
			"endpoint": "/ai2w/actions/book-table", "requires_auth": false, "requires_user_approval": true,
			"risk": "medium", "intent": "reserve_table",
			"bindings": []any{
				map[string]any{"kind": "mcp", "ref": "book_table", "priority": 1},
				map[string]any{"kind": "redirect", "ref": "/reserve", "priority": 9, "fallback_only": true},
			},
		}).
		Knowledge([]any{map[string]any{"id": "menu", "name": "Menu", "kind": "catalog", "ref": "/ai2w/products", "format": "json"}}).
		Governance(map[string]any{"rate_limits": map[string]any{"requests": 60, "window_seconds": 60}, "consent_mode": map[string]any{"book_table": "explicit"}}).
		UsagePolicy(map[string]any{"bulk_extraction": false, "model_training": false}).
		Legal(map[string]any{"jurisdiction": "EU", "ai_transparency": true, "ai_risk_classification": "limited"}).
		AgentIdentity(map[string]any{"required": false, "allow_anonymous": true, "methods": []any{"http_message_signatures"}}).
		Contact(map[string]any{"support": "hi@bistro.example"}).
		Build()

	if m["version"] != "0.2" {
		t.Errorf("version = %v", m["version"])
	}
	if m["governance"].(map[string]any)["rate_limits"].(map[string]any)["requests"] != 60 {
		t.Errorf("governance requests = %v", m["governance"])
	}
	if m["usage_policy"].(map[string]any)["model_training"] != false {
		t.Errorf("usage_policy = %v", m["usage_policy"])
	}
	if m["legal"].(map[string]any)["ai_risk_classification"] != "limited" {
		t.Errorf("legal = %v", m["legal"])
	}
	if m["identity"].(map[string]any)["agent"].(map[string]any)["methods"].([]any)[0] != "http_message_signatures" {
		t.Errorf("agent identity = %v", m["identity"])
	}
	if m["knowledge"].([]any)[0].(map[string]any)["id"] != "menu" {
		t.Errorf("knowledge = %v", m["knowledge"])
	}
	a0 := m["actions"].([]any)[0].(map[string]any)
	if a0["intent"] != "reserve_table" {
		t.Errorf("intent = %v", a0["intent"])
	}
	if len(a0["bindings"].([]any)) != 2 {
		t.Errorf("bindings = %v", a0["bindings"])
	}
	if a0["bindings"].([]any)[1].(map[string]any)["fallback_only"] != true {
		t.Errorf("fallback_only = %v", a0["bindings"])
	}

	txt := ToLlmsTxt(m)
	if !strings.HasPrefix(txt, "# Example Bistro") {
		t.Errorf("llms.txt title: %q", txt)
	}
	for _, want := range []string{"## Capabilities", "- commerce", "## Knowledge", "Menu", "book_table: Reserve a table.", "https://bistro.example/ai2w"} {
		if !strings.Contains(txt, want) {
			t.Errorf("llms.txt missing %q", want)
		}
	}

	aj := ToAgentJSON(m)
	if aj["name"] != "Example Bistro" {
		t.Errorf("agent.json name = %v", aj["name"])
	}
	if !slices.Contains(aj["capabilities"].([]string), "commerce") {
		t.Errorf("agent.json capabilities = %v", aj["capabilities"])
	}
	aja0 := aj["actions"].([]any)[0].(map[string]any)
	if aja0["intent"] != "reserve_table" {
		t.Errorf("agent.json intent = %v", aja0["intent"])
	}
	if len(aja0["bindings"].([]any)) != 2 {
		t.Errorf("agent.json bindings = %v", aja0["bindings"])
	}
	if aj["policies"].(map[string]any)["legal"].(map[string]any)["jurisdiction"] != "EU" {
		t.Errorf("agent.json legal = %v", aj["policies"])
	}
	if aj["policies"].(map[string]any)["governance"].(map[string]any)["consent_mode"].(map[string]any)["book_table"] != "explicit" {
		t.Errorf("agent.json governance = %v", aj["policies"])
	}

	// action without explicit bindings falls back to a rest binding on its endpoint
	aj2 := ToAgentJSON(ForSite("X", "https://x.example", "site").
		Action(map[string]any{"name": "a", "description": "d", "method": "POST", "endpoint": "/ai2w/actions/a", "requires_auth": false, "requires_user_approval": false, "risk": "low"}).
		Build())
	if aj2["actions"].([]any)[0].(map[string]any)["bindings"].([]any)[0].(map[string]any)["kind"] != "rest" {
		t.Errorf("default binding = %v", aj2["actions"])
	}
}

func TestMultiSurface(t *testing.T) {
	m := ForSite("Bistro", "https://bistro.example", "restaurant").
		Capability("content", nil).
		Capability("commerce", map[string]any{"endpoint": "/ai2w/products"}).
		Governance(map[string]any{"rate_limits": map[string]any{"requests": 60}}).
		Build()

	llms := Handle(ServerOptions{Manifest: m}, "GET", "/llms.txt", nil, "")
	if llms.Status != 200 {
		t.Fatalf("llms.txt status %d", llms.Status)
	}
	if ct := llms.Headers["content-type"]; !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("llms.txt content-type %q", ct)
	}
	if s, _ := llms.Body.(string); !strings.HasPrefix(s, "# Bistro") {
		t.Errorf("llms.txt body %v", llms.Body)
	}

	aj := Handle(ServerOptions{Manifest: m}, "GET", "/.well-known/agent.json", nil, "")
	if aj.Status != 200 {
		t.Fatalf("agent.json status %d", aj.Status)
	}
	if b, _ := aj.Body.(map[string]any); b["name"] != "Bistro" {
		t.Errorf("agent.json name %v", aj.Body)
	}
	if a2 := Handle(ServerOptions{Manifest: m}, "GET", "/agent.json", nil, ""); a2.Status != 200 {
		t.Errorf("agent.json alias status %d", a2.Status)
	}
	if post := Handle(ServerOptions{Manifest: m}, "POST", "/llms.txt", nil, ""); post.Status != 405 {
		t.Errorf("llms.txt POST expected 405, got %d", post.Status)
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
