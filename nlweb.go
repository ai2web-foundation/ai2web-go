package ai2web

// NLWeb (nlweb.ai) interop primitives.
//
// NLWeb turns a site's content into a natural-language, schema.org-flavoured query endpoint (its
// `ask` API). These helpers let an AI2Web site advertise an NLWeb surface in its manifest and
// serve a minimal, NLWeb-compatible `ask` response over its own content, so agents that speak
// NLWeb can query the site without it deploying the full NLWeb stack.
//
// The search itself is application-specific (a pure toolkit): the app finds the matching content
// items and passes them in; NlwebAskResponse shapes them into NLWeb's result envelope (list mode,
// schema.org Item results; pass an Answer for generate mode). NLWeb defines no discovery file, so
// NlwebTransport is an AI2Web convention pointing at the site's `/ask` (and `/mcp`) URLs.

const (
	NlwebVersion    = "0.55"
	nlwebDefaultAsk = "/ai2w/nlweb/ask"
	nlwebDefaultMCP = "/ai2w/nlweb/mcp"
)

// NlwebItemOptions carries defaults applied when a content item omits them.
type NlwebItemOptions struct {
	Site    string
	SiteURL string
}

// NlwebResponseOptions carries the optional fields of an ask response.
type NlwebResponseOptions struct {
	Site    string
	SiteURL string
	QueryID string
	Answer  string
}

// NlwebTransport returns the transports.nlweb advertisement to merge into a manifest.
func NlwebTransport(overrides map[string]any) map[string]any {
	t := map[string]any{
		"enabled": true,
		"version": NlwebVersion,
		"ask":     nlwebDefaultAsk,
		"mcp":     nlwebDefaultMCP,
		"modes":   []any{"list"},
	}
	for k, v := range overrides {
		t[k] = v
	}
	return t
}

// NlwebItem wraps one content item into an NLWeb result Item.
func NlwebItem(content map[string]any, opts NlwebItemOptions) map[string]any {
	name := nlStr(content, "name")
	if name == "" {
		name = nlStr(content, "title")
	}
	site := nlStr(content, "site")
	if site == "" {
		site = opts.Site
	}
	siteURL := nlStr(content, "siteUrl")
	if siteURL == "" {
		siteURL = opts.SiteURL
	}
	score := 100
	switch s := content["score"].(type) {
	case int:
		score = s
	case int64:
		score = int(s)
	case float64:
		score = int(s)
	}
	var schema map[string]any
	if so, ok := content["schema_object"].(map[string]any); ok {
		schema = so
	} else {
		schema = nlwebSchemaObject(content)
	}
	return map[string]any{
		"@type":         "Item",
		"url":           nlStr(content, "url"),
		"name":          name,
		"site":          site,
		"siteUrl":       siteURL,
		"score":         score,
		"description":   nlStr(content, "description"),
		"schema_object": schema,
	}
}

// NlwebAskResponse builds a minimal buffered NLWeb ask response (list mode) from matched items.
func NlwebAskResponse(query string, items []map[string]any, opts NlwebResponseOptions) map[string]any {
	results := make([]any, 0, len(items))
	for _, it := range items {
		results = append(results, NlwebItem(it, NlwebItemOptions{Site: opts.Site, SiteURL: opts.SiteURL}))
	}
	qid := opts.QueryID
	if qid == "" {
		qid = "q_" + ap2RandHex(8)
	}
	resp := map[string]any{
		"query":        query,
		"query_id":     qid,
		"message_type": "result",
		"results":      results,
	}
	if opts.Answer != "" {
		resp["answer"] = map[string]any{"@type": "GeneratedAnswer", "answer": opts.Answer, "items": results}
	}
	return resp
}

func nlwebSchemaObject(c map[string]any) map[string]any {
	t := nlStr(c, "type")
	if t == "" {
		t = "Thing"
	}
	obj := map[string]any{"@type": t}
	name := nlStr(c, "name")
	if name == "" {
		name = nlStr(c, "title")
	}
	if name != "" {
		obj["name"] = name
	}
	if u := nlStr(c, "url"); u != "" {
		obj["url"] = u
	}
	if d := nlStr(c, "description"); d != "" {
		obj["description"] = d
	}
	return obj
}

func nlStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
