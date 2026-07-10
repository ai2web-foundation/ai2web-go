package ai2web

import "regexp"

var versionRe = regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)

// Check is one scored line of an AI Readiness evaluation.
type Check struct {
	OK     bool    `json:"ok"`
	Points int     `json:"points"`
	Label  string  `json:"label"`
	Hint   *string `json:"hint"`
}

// Result is the output of Validate.
type Result struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
	Checks []Check  `json:"checks"`
	Score  int      `json:"score"`
	Tier   string   `json:"tier"`
}

// Validate scores a manifest and computes the AI Readiness Score + compliance tier.
// Port of @ai2web/core validateManifest (spec §9/§11). Parity with cases.json.
func Validate(m Manifest) Result {
	errs := []string{}
	checks := []Check{}
	caps := toMap(m["capabilities"])
	capOf := func(n string) any {
		if caps == nil {
			return nil
		}
		return caps[n]
	}

	if m["protocol"] != "ai2w" {
		errs = append(errs, "protocol must be 'ai2w'")
	}
	if !versionRe.MatchString(toStr(m["version"])) {
		errs = append(errs, "version missing/invalid")
	}
	site := toMap(m["site"])
	for _, k := range []string{"name", "url", "type"} {
		if toStr(site[k]) == "" {
			errs = append(errs, "site."+k+" missing")
		}
	}
	if len(caps) == 0 {
		errs = append(errs, "capabilities empty")
	}

	actionsExist := has(capOf("actions")) || len(toSlice(m["actions"])) > 0 || has(capOf("commerce")) || has(capOf("booking"))

	score := 0
	add := func(ok bool, points int, label, hint string) {
		var h *string
		if !ok {
			hh := hint
			h = &hh
		}
		checks = append(checks, Check{OK: ok, Points: points, Label: label, Hint: h})
		if ok {
			score += points
		}
	}

	add(len(errs) == 0, 30, "Valid discovery manifest", "fix errors")
	add(has(capOf("content")), 6, "Content", "expose content module")
	add(has(capOf("commerce")) || has(capOf("booking")) || has(capOf("services")), 6, "Products / services / booking", "expose a commerce/services/booking module")
	add(has(capOf("search")), 4, "Search", "add a search capability")
	add(actionsExist, 5, "Actions", "declare actions")
	add(has(capOf("events")), 6, "Events / subscriptions", "publish subscribable events")
	add(boolInMap(m["agent_service"], "enabled"), 4, "Agent service (A2A)", "expose /ai2w/agent")

	commerce := capOf("commerce")
	add(!has(commerce) || boolInMap(commerce, "checkout"), 4, "Checkout", "commerce present but checkout missing")

	tr := toMap(m["transports"])
	add(boolInMap(tr["mcp"], "enabled"), 8, "MCP transport", "expose an MCP endpoint")
	add(boolInMap(tr["rest"], "enabled") || tr["feeds"] != nil, 4, "REST / feeds", "expose REST or feeds")

	auth := toMap(m["auth"])
	oauthOk := containsStr(toSlice(auth["methods"]), "oauth2") && boolInMap(auth["oauth2"], "pkce")
	consentDeclared := len(toSlice(toMap(m["consent"])["requires_user_approval_for"])) > 0
	add(!actionsExist || oauthOk, 8, "OAuth2 + PKCE", "protected actions need oauth2+pkce")
	add(!actionsExist || consentDeclared, 7, "Consent declared", "declare consent for sensitive actions")

	add(m["identity"] != nil, 4, "Identity", "add identity (legal_name, policies)")
	add(m["contact"] != nil, 4, "Contact", "add support/security contact")

	if score > 100 {
		score = 100
	}

	basic := len(errs) == 0
	standard := basic && m["transports"] != nil && (!actionsExist || consentDeclared) && m["contact"] != nil
	enterprise := standard && m["identity"] != nil && m["auth"] != nil && m["rate_limits"] != nil
	tier := "Invalid"
	switch {
	case enterprise:
		tier = "Enterprise"
	case standard:
		tier = "Standard"
	case basic:
		tier = "Basic"
	}

	return Result{Valid: len(errs) == 0, Errors: errs, Checks: checks, Score: score, Tier: tier}
}
