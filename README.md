<div align="center">
  <a href="https://ai2web.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/ai2web-foundation/.github/main/profile/ai2web-logo-white.svg">
      <img alt="AI2Web" src="https://raw.githubusercontent.com/ai2web-foundation/.github/main/profile/ai2web-logo-black.svg" width="200">
    </picture>
  </a>
</div>

# AI2Web Go SDK (`github.com/ai2web-foundation/ai2web-go`)

[![AI2Web on Launchpadly - Product of the Week (Gold)](https://launchpadly.co/embed/badges/startup/ai2web.svg?variant=product-week-gold)](https://launchpadly.co/startup/ai2web?ref=badge)

[![CI](https://github.com/ai2web-foundation/ai2web-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ai2web-foundation/ai2web-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ai2web-foundation/ai2web-go.svg)](https://pkg.go.dev/github.com/ai2web-foundation/ai2web-go)

The Go reference implementation of the [AI2Web protocol](https://github.com/ai2web-foundation/ai2web-spec). Mirrors `@ai2web/core`.

```bash
go get github.com/ai2web-foundation/ai2web-go
```

```go
package main

import (
	"fmt"
	ai2web "github.com/ai2web-foundation/ai2web-go"
)

func main() {
	m := ai2web.ForSite("Example Store", "https://example.com", "ecommerce").
		Capability("content", nil).
		Capability("commerce", map[string]any{"endpoint": "/ai2w/products", "checkout": true}).
		Transports(map[string]any{"mcp": map[string]any{"enabled": true, "endpoint": "/ai2w/mcp"}, "rest": map[string]any{"enabled": true}}).
		Auth(map[string]any{"methods": []any{"none", "oauth2"}, "oauth2": map[string]any{"pkce": true}}).
		Consent(map[string]any{"requires_user_approval_for": []any{"purchase"}}).
		Contact(map[string]any{"support": "help@example.com"}).
		Build()

	r := ai2web.Validate(m)                       // Result{Score, Tier, Valid, ...}
	fmt.Println(r.Score, r.Tier)

	// Serve every AI2Web route (framework-agnostic):
	res := ai2web.Handle(ai2web.ServerOptions{Manifest: m}, method, path, body, origin)
	_ = res
}
```

## API
- `New` / `ForSite` → `*Builder` - fluent capability-model builder.
- `Validate(Manifest) Result` - AI Readiness scoring (spec §9/§11).
- `Negotiate(Manifest, agent) Negotiation` - capability negotiation (spec §5).
- `Handle(ServerOptions, method, path, body, origin) Response` - framework-agnostic router.
- `IsSafePublicURL` / `AssertSafePublicURL` / `SameOrigin` - SSRF guard.

## Test
```bash
go test ./...     # includes the shared conformance contract (testdata/conformance_cases.json)
```

Requires **Go 1.21+**. No external dependencies.

## Licence
MIT.
