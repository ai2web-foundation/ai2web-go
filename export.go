package ai2web

import "strings"

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
