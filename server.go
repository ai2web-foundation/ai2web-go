package ai2web

import (
	"regexp"
	"strings"
)

// Response is a transport-agnostic AI2Web response. Adapt to net/http, etc.
type Response struct {
	Status  int
	Headers map[string]string
	Body    any
}

// Handler is a module/action handler.
type Handler func(body any) any

// ServerOptions configures Handle.
type ServerOptions struct {
	Manifest Manifest
	Modules  map[string]Handler
	Actions  map[string]Handler
	// ValidateInput validates each action's request body against its declared input_schema
	// before the handler runs (returns 400 invalid_request on mismatch). Defaults to on;
	// set to a pointer to false to opt out.
	ValidateInput *bool
}

var corsHeaders = map[string]string{
	"access-control-allow-origin":  "*",
	"access-control-allow-methods": "GET, POST, OPTIONS",
	"access-control-allow-headers": "content-type, authorization",
}

var actionRe = regexp.MustCompile(`(?i)^/ai2w/actions/([a-z0-9_-]+)$`)
var moduleRe = regexp.MustCompile(`(?i)^/ai2w/([a-z0-9_-]+)$`)

func jsonResp(status int, body any) Response {
	h := map[string]string{"content-type": "application/json; charset=utf-8"}
	for k, v := range corsHeaders {
		h[k] = v
	}
	return Response{Status: status, Headers: h, Body: body}
}

func errResp(status int, code, message string) Response {
	return jsonResp(status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": false}})
}

func textResp(status int, contentType, body string) Response {
	h := map[string]string{"content-type": contentType}
	for k, v := range corsHeaders {
		h[k] = v
	}
	return Response{Status: status, Headers: h, Body: body}
}

// Handle serves an AI2Web request: manifest, well-known anchor, negotiation, and module/action dispatch.
func Handle(opts ServerOptions, method, path string, body any, origin string) Response {
	if p := strings.Trim(path, "/"); p == "" {
		path = "/"
	} else {
		path = "/" + p
	}
	method = strings.ToUpper(method)

	if method == "OPTIONS" {
		return Response{Status: 204, Headers: corsHeaders, Body: nil}
	}
	if path == "/.well-known/ai2w" {
		if origin != "" {
			return jsonResp(200, map[string]any{"ai2w": strings.TrimRight(origin, "/") + "/ai2w"})
		}
		return jsonResp(200, opts.Manifest)
	}
	if path == "/ai2w" || path == "/ai" || path == "/.ai" {
		if method != "GET" {
			return errResp(405, "invalid_request", "Use GET for the manifest.")
		}
		return jsonResp(200, opts.Manifest)
	}
	// Multi-surface projections (RFC-0015): the one canonical manifest, emitted in other
	// discovery formats so agents that speak llms.txt or agent.json need not parse ai2w first.
	if path == "/llms.txt" {
		if method != "GET" {
			return errResp(405, "invalid_request", "Use GET for llms.txt.")
		}
		return textResp(200, "text/plain; charset=utf-8", ToLlmsTxt(opts.Manifest))
	}
	if path == "/.well-known/agent.json" || path == "/agent.json" {
		if method != "GET" {
			return errResp(405, "invalid_request", "Use GET for agent.json.")
		}
		return jsonResp(200, ToAgentJSON(opts.Manifest))
	}
	if path == "/ai2w/negotiate" {
		supports := map[string]any{}
		if bm := toMap(body); bm != nil {
			if a := toMap(bm["agent"]); a != nil && a["supports"] != nil {
				supports = toMap(a["supports"])
			} else if s := toMap(bm["supports"]); s != nil {
				supports = s
			} else {
				supports = bm
			}
		}
		return jsonResp(200, Negotiate(opts.Manifest, supports))
	}
	if mm := actionRe.FindStringSubmatch(path); mm != nil {
		name := strings.ReplaceAll(mm[1], "-", "_")
		fn, ok := opts.Actions[name]
		if !ok {
			return errResp(404, "unsupported_capability", "Unknown action '"+name+"'.")
		}
		if opts.ValidateInput == nil || *opts.ValidateInput {
			if schema := actionInputSchema(opts.Manifest, name); schema != nil {
				b := body
				if b == nil {
					b = map[string]any{}
				}
				if valid, errs := ValidateSchema(b, schema, "input"); !valid {
					return errResp(400, "invalid_request", "Request does not match the declared input schema: "+strings.Join(errs, "; ")+".")
				}
			}
		}
		return jsonResp(200, fn(body))
	}
	if mm := moduleRe.FindStringSubmatch(path); mm != nil {
		name := mm[1]
		if fn, ok := opts.Modules[name]; ok {
			return jsonResp(200, fn(body))
		}
		return errResp(404, "unsupported_capability", "Module '"+name+"' not exposed.")
	}
	return errResp(404, "invalid_request", "No AI2Web route for "+path+".")
}
