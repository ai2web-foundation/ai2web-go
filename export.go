package ai2web

import (
	"net/url"
	"strings"
)

// Export adapters (RFC-0015): project the one canonical AI2Web manifest into other wire formats
// and discovery surfaces. Mirrors @ai2web/core's export.ts.
//
// Each export is a best-effort projection; where a target cannot represent a field, it is omitted
// rather than misstated. The canonical /ai2w manifest stays authoritative for execution.

func enabledCapabilities(m Manifest) []string {
	out := []string{}
	for k, v := range toMap(m["capabilities"]) {
		if has(v) {
			out = append(out, k)
		}
	}
	return out
}

// ToLlmsTxt projects the manifest to an llms.txt document: a plain-text summary and links a model
// can read for content and guidance. Reads only; no actions are exposed here.
func ToLlmsTxt(m Manifest) string {
	site := toMap(m["site"])
	base := strings.TrimRight(toStr(site["url"]), "/")
	lines := []string{"# " + toStr(site["name"])}
	if d := toStr(site["description"]); d != "" {
		lines = append(lines, "", "> "+d)
	}

	if caps := enabledCapabilities(m); len(caps) > 0 {
		lines = append(lines, "", "## Capabilities")
		for _, c := range caps {
			lines = append(lines, "- "+c)
		}
	}

	if kn := toSlice(m["knowledge"]); len(kn) > 0 {
		lines = append(lines, "", "## Knowledge")
		for _, ki := range kn {
			k := toMap(ki)
			ref := toStr(k["ref"])
			if !strings.HasPrefix(ref, "http") {
				sep := "/"
				if strings.HasPrefix(ref, "/") {
					sep = ""
				}
				ref = base + sep + ref
			}
			name := toStr(k["name"])
			if name == "" {
				name = toStr(k["id"])
			}
			lines = append(lines, "- ["+name+"]("+ref+")")
		}
	}

	if acts := toSlice(m["actions"]); len(acts) > 0 {
		lines = append(lines, "", "## Actions")
		for _, ai := range acts {
			a := toMap(ai)
			lines = append(lines, "- "+toStr(a["name"])+": "+toStr(a["description"]))
		}
	}

	lines = append(lines, "", "## Discovery", "- Manifest: "+base+"/ai2w")
	return strings.Join(lines, "\n") + "\n"
}

// ToAgentJSON projects the manifest to a generic agent.json style capability document. Best-effort,
// format-neutral projection of identity, capabilities, actions (with bindings), knowledge and
// policies. Consent/governance a target cannot express are carried as a policies object.
func ToAgentJSON(m Manifest) map[string]any {
	site := toMap(m["site"])
	consent := toMap(m["consent"])

	actions := []any{}
	for _, ai := range toSlice(m["actions"]) {
		a := toMap(ai)
		var bindings any = a["bindings"]
		if bindings == nil {
			bindings = []any{map[string]any{"kind": "rest", "ref": a["endpoint"]}}
		}
		actions = append(actions, map[string]any{
			"name":             a["name"],
			"intent":           a["intent"],
			"description":      a["description"],
			"risk":             a["risk"],
			"requires_consent": a["requires_user_approval"],
			"requires_auth":    a["requires_auth"],
			"input_schema":     a["input_schema"],
			"bindings":         bindings,
		})
	}

	return map[string]any{
		"schema":       "agent-capabilities",
		"name":         site["name"],
		"description":  site["description"],
		"url":          site["url"],
		"identity":     m["identity"],
		"capabilities": enabledCapabilities(m),
		"actions":      actions,
		"knowledge":    m["knowledge"],
		"transports":   m["transports"],
		"policies": map[string]any{
			"consent":    consent["requires_user_approval_for"],
			"governance": m["governance"],
			"usage":      m["usage_policy"],
			"legal":      m["legal"],
		},
	}
}

// ToOAuthProtectedResource projects the manifest to OAuth 2.0 Protected Resource metadata
// (RFC 9728), for /.well-known/oauth-protected-resource. MCP clients read this to discover which
// authorization server guards the resource before starting a flow.
//
// Returns nil when the site does not advertise oauth2, so an auth surface the site cannot honour
// is never published.
func ToOAuthProtectedResource(m Manifest) map[string]any {
	auth := toMap(m["auth"])
	oauth := false
	for _, v := range toSlice(auth["methods"]) {
		if toStr(v) == "oauth2" {
			oauth = true
			break
		}
	}
	if !oauth {
		return nil
	}
	base := strings.TrimRight(toStr(toMap(m["site"])["url"]), "/")
	issuer := base
	o2 := toMap(auth["oauth2"])
	if authz := toStr(o2["authorization_url"]); authz != "" {
		if u, err := url.Parse(authz); err == nil && u.Scheme != "" && u.Host != "" {
			issuer = u.Scheme + "://" + u.Host
		}
	}
	doc := map[string]any{
		"resource":                 base + "/ai2w",
		"authorization_servers":    []string{issuer},
		"bearer_methods_supported": []string{"header"},
	}
	if scopes := toSlice(o2["scopes"]); len(scopes) > 0 {
		out := make([]string, 0, len(scopes))
		for _, s := range scopes {
			out = append(out, toStr(s))
		}
		doc["scopes_supported"] = out
	}
	return doc
}

// ToContentSignals maps usage_policy onto Content Signals tokens. `search` stays yes because
// AI2Web exists to be discoverable; the AI signals are only asserted when the manifest states
// them, so an unset policy is never reported as a refusal. Empty string when no policy exists.
func ToContentSignals(m Manifest) string {
	p := toMap(m["usage_policy"])
	if len(p) == 0 {
		return ""
	}
	signals := []string{"search=yes"}
	if v, ok := p["content_reproduction"].(bool); ok {
		signals = append(signals, "ai-input="+yesNo(v))
	}
	if v, ok := p["model_training"].(bool); ok {
		signals = append(signals, "ai-train="+yesNo(v))
	}
	return strings.Join(signals, ", ")
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// ToRobotsTxt returns a robots.txt FRAGMENT carrying the usage policy and a pointer to the
// manifest. Append it to an existing robots.txt; it is never a replacement and emits no Disallow.
func ToRobotsTxt(m Manifest) string {
	base := strings.TrimRight(toStr(toMap(m["site"])["url"]), "/")
	lines := []string{"# AI2Web usage policy, projected from " + base + "/ai2w", "User-agent: *"}
	if s := ToContentSignals(m); s != "" {
		lines = append(lines, "Content-Signal: "+s)
	}
	if v, ok := toMap(m["usage_policy"])["bulk_extraction"].(bool); ok && !v {
		lines = append(lines, "# bulk_extraction: false - please use the /ai2w endpoints instead of crawling")
	}
	lines = append(lines, "# AI2Web-Manifest: "+base+"/ai2w")
	return strings.Join(lines, "\n") + "\n"
}

// ToDiscoveryLinkHeader is the value for an HTTP Link header advertising the manifest, so
// non-HTML clients discover it without parsing a page for <link rel="ai2w">.
func ToDiscoveryLinkHeader(m Manifest) string {
	return "<" + strings.TrimRight(toStr(toMap(m["site"])["url"]), "/") + "/ai2w>; rel=\"ai2w\""
}
