package ai2web

import "sort"

// Negotiated is the agreed capability set + transport.
type Negotiated struct {
	Transport    *string           `json:"transport"`
	Capabilities []string          `json:"capabilities"`
	Auth         *string           `json:"auth"`
	Endpoints    map[string]string `json:"endpoints"`
}

// Negotiation is the result of Negotiate.
type Negotiation struct {
	Negotiated  Negotiated `json:"negotiated"`
	Unsupported []string   `json:"unsupported"`
}

func endpointOf(name string, v any) string {
	if m := toMap(v); m != nil {
		if s, ok := m["endpoint"].(string); ok {
			return s
		}
	}
	return "/ai2w/" + name
}

// Negotiate agrees a capability set + transport (spec §5). Capability/unsupported lists
// are sorted (a set - order is not normative) because Go map iteration is unordered.
func Negotiate(m Manifest, agent map[string]any) Negotiation {
	if agent == nil {
		agent = map[string]any{}
	}
	caps := toMap(m["capabilities"])
	siteSet := map[string]bool{}
	siteList := []string{}
	for k, v := range caps {
		if has(v) {
			siteSet[k] = true
			siteList = append(siteList, k)
		}
	}

	var want []string
	if wc, ok := agent["capabilities"]; ok && wc != nil {
		for _, v := range toSlice(wc) {
			want = append(want, toStr(v))
		}
	} else {
		want = siteList
	}
	capsOut, unsupported := []string{}, []string{}
	for _, c := range want {
		if siteSet[c] {
			capsOut = append(capsOut, c)
		} else {
			unsupported = append(unsupported, c)
		}
	}
	sort.Strings(capsOut)
	sort.Strings(unsupported)

	// Transports: only enabled ones are negotiable.
	tr := toMap(m["transports"])
	transportSet := map[string]bool{}
	for k, v := range tr {
		if boolInMap(v, "enabled") {
			transportSet[k] = true
		}
	}
	var wantT []string
	if wt, ok := agent["transports"]; ok && wt != nil {
		for _, v := range toSlice(wt) {
			wantT = append(wantT, toStr(v))
		}
	} else {
		for k := range transportSet {
			wantT = append(wantT, k)
		}
		sort.Strings(wantT)
	}
	var transport *string
	for _, t := range wantT {
		if transportSet[t] {
			tt := t
			transport = &tt
			break
		}
	}

	// Auth: prefer oauth2 if both support it.
	siteAuth := toSlice(toMap(m["auth"])["methods"])
	if len(siteAuth) == 0 {
		siteAuth = []any{"none"}
	}
	var wantA []any
	if wa, ok := agent["auth"]; ok && wa != nil {
		wantA = toSlice(wa)
	} else {
		wantA = siteAuth
	}
	var auth *string
	if containsStr(siteAuth, "oauth2") && containsStr(wantA, "oauth2") {
		s := "oauth2"
		auth = &s
	} else {
		for _, a := range wantA {
			if containsStr(siteAuth, toStr(a)) {
				s := toStr(a)
				auth = &s
				break
			}
		}
		if auth == nil && containsStr(siteAuth, "none") {
			s := "none"
			auth = &s
		}
	}

	endpoints := map[string]string{}
	for _, c := range capsOut {
		endpoints[c] = endpointOf(c, caps[c])
	}
	if transport != nil {
		if tm := toMap(tr[*transport]); tm != nil {
			if ep, ok := tm["endpoint"].(string); ok {
				endpoints[*transport] = ep
			}
		}
	}

	return Negotiation{
		Negotiated:  Negotiated{Transport: transport, Capabilities: capsOut, Auth: auth, Endpoints: endpoints},
		Unsupported: unsupported,
	}
}
