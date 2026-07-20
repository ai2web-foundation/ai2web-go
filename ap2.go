package ai2web

// AP2 (Agent Payments Protocol, Google - v0.2.0) merchant primitives.
//
// AP2 is mandate-based: the merchant prices a buyer agent's Intent Mandate as a CartContents
// (a W3C PaymentRequest, amounts in decimal major units) and digitally signs it into a
// CartMandate - a short-lived guarantee of items and price - then settles a user-signed Payment
// Mandate. These are the reusable, app-agnostic merchant primitives: build the mandate objects,
// sign a CartContents as an RS256 JWT (cart_hash over the canonical contents), publish the public
// key as a JWKS, verify a Cart Mandate, and parse a Payment Mandate. Signing uses the standard
// library (crypto/rsa), so the SDK keeps zero third-party dependencies.

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"
)

const (
	AP2ExtensionURI = "https://github.com/google-agentic-commerce/ap2/v1"
	AP2Version      = "0.2.0"
	ap2DefaultTTL   = int64(900)
)

// AP2Options carries optional signing/build parameters. Zero values mean "use the default".
type AP2Options struct {
	Now              int64 // 0 = time.Now().Unix()
	ExpiresIn        int64 // 0 = 900
	Kid              string
	Iss              string
	Aud              string
	ID               string
	PaymentDetailsID string
}

// AP2LineItem is one cart line: a label, a unit price and a quantity (default 1).
type AP2LineItem struct {
	Label      string
	UnitAmount float64
	Quantity   int
}

// AP2IntentOptions carries the optional fields of an Intent Mandate.
type AP2IntentOptions struct {
	Merchants                    []string
	Skus                         []string
	Items                        []map[string]any
	RequiresRefundability        bool
	UserCartConfirmationRequired *bool
	ExpiresIn                    int64
	Now                          int64
}

// AP2Transport returns the transports.ap2 advertisement to merge into a manifest.
func AP2Transport(overrides map[string]any) map[string]any {
	t := map[string]any{
		"enabled":    true,
		"version":    AP2Version,
		"extension":  AP2ExtensionURI,
		"agent_card": "/ai2w/ap2/agent-card",
		"cart":       "/ai2w/ap2/cart",
		"payment":    "/ai2w/ap2/payment",
		"jwks":       "/ai2w/ap2/jwks",
	}
	for k, v := range overrides {
		t[k] = v
	}
	return t
}

// AP2IntentMandate builds an AP2 IntentMandate (classic v0.2.0 shape).
func AP2IntentMandate(description string, opts AP2IntentOptions) map[string]any {
	now := opts.Now
	if now == 0 {
		now = time.Now().Unix()
	}
	ttl := opts.ExpiresIn
	if ttl == 0 {
		ttl = ap2DefaultTTL
	}
	ucc := true
	if opts.UserCartConfirmationRequired != nil {
		ucc = *opts.UserCartConfirmationRequired
	}
	m := map[string]any{
		"natural_language_description":    description,
		"intent_expiry":                   ap2ISO(now + ttl),
		"user_cart_confirmation_required": ucc,
	}
	if len(opts.Merchants) > 0 {
		m["merchants"] = opts.Merchants
	}
	if len(opts.Skus) > 0 {
		m["skus"] = opts.Skus
	}
	if len(opts.Items) > 0 {
		m["items"] = opts.Items
	}
	if opts.RequiresRefundability {
		m["requires_refundability"] = true
	}
	return m
}

// AP2Amount is an AP2 PaymentCurrencyAmount: decimal major units, ISO 4217.
func AP2Amount(value float64, currency string) map[string]any {
	return map[string]any{"currency": strings.ToUpper(currency), "value": math.Round(value*100) / 100}
}

// AP2CartContents builds a CartContents (W3C PaymentRequest) from line items.
func AP2CartContents(items []AP2LineItem, currency, merchantName string, opts AP2Options) map[string]any {
	now := opts.Now
	if now == 0 {
		now = time.Now().Unix()
	}
	ttl := opts.ExpiresIn
	if ttl == 0 {
		ttl = ap2DefaultTTL
	}
	display := make([]any, 0, len(items))
	total := 0.0
	for _, it := range items {
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		line := it.UnitAmount * float64(qty)
		label := it.Label
		if qty > 1 {
			label = fmt.Sprintf("%s x%d", label, qty)
		}
		display = append(display, map[string]any{"label": label, "amount": AP2Amount(line, currency)})
		total += line
	}
	id := opts.ID
	if id == "" {
		id = "cart_" + ap2RandHex(10)
	}
	pdid := opts.PaymentDetailsID
	if pdid == "" {
		pdid = "pr_" + ap2RandHex(10)
	}
	return map[string]any{
		"id":                              id,
		"user_cart_confirmation_required": true,
		"payment_request": map[string]any{
			"method_data": []any{map[string]any{"supported_methods": "card", "data": map[string]any{}}},
			"details": map[string]any{
				"id":            pdid,
				"display_items": display,
				"total":         map[string]any{"label": "Total", "amount": AP2Amount(total, currency)},
			},
			"options": map[string]any{"request_shipping": true},
		},
		"cart_expiry":   ap2ISO(now + ttl),
		"merchant_name": merchantName,
	}
}

// AP2SignCart returns the merchant_authorization JWT (RS256) over the canonical CartContents.
func AP2SignCart(contents map[string]any, privateKeyPEM string, opts AP2Options) (string, error) {
	key, err := ap2ParsePrivate(privateKeyPEM)
	if err != nil {
		return "", err
	}
	now := opts.Now
	if now == 0 {
		now = time.Now().Unix()
	}
	ttl := opts.ExpiresIn
	if ttl == 0 {
		ttl = ap2DefaultTTL
	}
	kid := opts.Kid
	if kid == "" {
		kid = ap2Kid(&key.PublicKey)
	}
	iss := opts.Iss
	if iss == "" {
		if s, ok := contents["merchant_name"].(string); ok {
			iss = s
		}
	}
	aud := opts.Aud
	if aud == "" {
		aud = "ap2-network"
	}
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
	claims := map[string]any{
		"iss":       iss,
		"sub":       contents["id"],
		"aud":       aud,
		"iat":       now,
		"exp":       now + ttl,
		"jti":       ap2RandHex(12),
		"cart_hash": ap2B64url(ap2Sha256(ap2Canonical(contents))),
	}
	signingInput := ap2B64url(ap2Canonical(header)) + "." + ap2B64url(ap2Canonical(claims))
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + ap2B64url(sig), nil
}

// AP2CartMandate signs CartContents into a CartMandate (contents + merchant_authorization).
func AP2CartMandate(contents map[string]any, privateKeyPEM string, opts AP2Options) (map[string]any, error) {
	jwt, err := AP2SignCart(contents, privateKeyPEM, opts)
	if err != nil {
		return nil, err
	}
	return map[string]any{"contents": contents, "merchant_authorization": jwt}, nil
}

// AP2JWKS publishes the cart-signing public key, for verifiers.
func AP2JWKS(privateKeyPEM, kid string) (map[string]any, error) {
	key, err := ap2ParsePrivate(privateKeyPEM)
	if err != nil {
		return map[string]any{"keys": []any{}}, err
	}
	pub := &key.PublicKey
	if kid == "" {
		kid = ap2Kid(pub)
	}
	return map[string]any{"keys": []any{map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": kid,
		"n":   ap2B64url(pub.N.Bytes()),
		"e":   ap2B64url(big.NewInt(int64(pub.E)).Bytes()),
	}}}, nil
}

// AP2VerifyCartMandate verifies a CartMandate's signature (against a public or private PEM) and
// its cart_hash binding, and that it has not expired.
func AP2VerifyCartMandate(mandate map[string]any, keyPEM string) bool {
	jwt, _ := mandate["merchant_authorization"].(string)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return false
	}
	pub, err := ap2ParsePublic(keyPEM)
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig) != nil {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return false
	}
	ch, _ := claims["cart_hash"].(string)
	if ch == "" {
		return false
	}
	if exp, ok := claims["exp"].(float64); ok && time.Now().Unix() > int64(exp) {
		return false
	}
	contents, _ := mandate["contents"].(map[string]any)
	expected := ap2B64url(ap2Sha256(ap2Canonical(contents)))
	return ch == expected
}

// AP2PaymentDetails extracts the salient fields of a PaymentMandate for settlement.
func AP2PaymentDetails(paymentMandate map[string]any) map[string]any {
	c, _ := paymentMandate["payment_mandate_contents"].(map[string]any)
	if c == nil {
		c = map[string]any{}
	}
	resp, _ := c["payment_response"].(map[string]any)
	if resp == nil {
		resp = map[string]any{}
	}
	var amt any
	if total, ok := c["payment_details_total"].(map[string]any); ok {
		amt = total["amount"]
	}
	return map[string]any{
		"payment_mandate_id": c["payment_mandate_id"],
		"payment_details_id": c["payment_details_id"],
		"total":              amt,
		"method":             resp["method_name"],
		"payer_email":        resp["payer_email"],
		"payer_name":         resp["payer_name"],
	}
}

// --- helpers ---

func ap2Canonical(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func ap2B64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func ap2Sha256(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

func ap2RandHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ap2ISO(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02T15:04:05+00:00")
}

func ap2Kid(pub *rsa.PublicKey) string {
	e := big.NewInt(int64(pub.E)).Bytes()
	h := sha256.Sum256(append(append([]byte{}, pub.N.Bytes()...), e...))
	return hex.EncodeToString(h[:])[:16]
}

func ap2ParsePrivate(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("ap2: invalid PEM")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
	}
	if rk, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rk, nil
	}
	return nil, fmt.Errorf("ap2: not an RSA private key")
}

func ap2ParsePublic(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block != nil {
		if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
			if pk, ok := k.(*rsa.PublicKey); ok {
				return pk, nil
			}
		}
		if pk, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
			return pk, nil
		}
	}
	// Fall back to deriving the public key from a private-key PEM.
	if priv, err := ap2ParsePrivate(pemStr); err == nil {
		return &priv.PublicKey, nil
	}
	return nil, fmt.Errorf("ap2: not an RSA key")
}
