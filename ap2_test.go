package ai2web

import "testing"

const ap2TestKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC7/yKHuyEHpcRo
Zahdi0IJeDyBoy7jV73flum/ysm3H3nK1lh7WHPNV1r27rOodKAIiJH/yVrKcAeR
qRyDgJ8ftAIla/qj9zDu3h5rR40wRDM60DhpkjMoHa2aQ3Lh93wH004k40HxvWOA
FORAZPrxo4JJTA7Qayak4VwWH2zepeSpmqO3kovZR4DDeDRJf/UnWC5fDAvQno+W
c2lVdbzeErLS1TvbmVDVfIwPkE008gZWEhQ/qK3RSoQEUxqeqaA8BM/WYdQr+PDv
EJgT0MfECcV+6ACMNHTCzspVRkE3pPcM2PVJekbGlirzxYMn2i0Hs0xgz1lwjEAb
/pIA3Vh1AgMBAAECggEAGRI5ZKiMCx0MSG/mODNuJx0l1JQSmLcG116k5bMBm65S
674SJsDxEJ1pwCytQPXssbak4dvUg9LU75QB/XeVwQCcmKkB0AQTPofYvq3YImu1
+U3zeADLWbo7gKsmEwSSQejoLvsvvDFpp5chqYTOApOvuF6wSxM/IBX91eVy+24h
sQgxxwmYtwaFqiW56oNcF+8OZVCenZF4NWGfJ6vDxyIgkfvlhPSzQl8BimzIB2j+
hs5S4TYY1fE7pcuI91zk2dGpK9E1nxl3e57gZJ19w+YrhOXvatOSX++QeBrv2Vik
kU1SbJq5K3fcGvjkEYXRqth0loTbZl3HxOgef4QksQKBgQDlxWxaQFrsa5pFP1a2
iklsuIbKr/0DgHuENzlZtrUPzbzCYBQT28ADa+3HZIvXvNo4bUbHayrrwQh6nFWl
n0JUVl3JzUcGJO6nJH/4uLI/G4NkMz/BW5G1fMnfpEBc2LAWbGYE0tgFxL/uvTeL
o5zTI3ElZX5FsMb/KAoU5J8TYwKBgQDRdPA5ydXMoooQQ3mYc/UUdnVZPtiN0G1j
+v/QyH5+0SEbj5AUaIbuTblNANRZsiz0OjJ4i5ZrXLRXOwYL0WvcC2we1KnRaomv
dNmdQwu31YRnxEq97/3dSBJC7K0VkiRjrLIZD/dDDUnjFjBD1fa51AcedXmPJNjf
3RyTYcKoRwKBgQDh8x2VNtnyyfHADQQ5p42C04cBxMSbb/qGz0OffHNbIidwQckc
qimNc9I1FSQLuBQkDxneOv3PLlknMZtrrkws4W2DaFFismjZhqQts3rdYjH4FAmr
HGASR6/BNCVy6EdpFZnRPoHeUlen7vyzXeZ3HtBCRSdCYw+dlQMs/pGMHwKBgQCG
igaEGBEskHr+V1kTg+g4bJ6T5LpU3TxmrCMFiMM30jzh5yU09q81AtezjoTX2Irn
lTo2E/NaowFzxoXrsWkGvo+EfjVWPoiSGwxs51PvkUarIHqh5jW6nUCdnEjRQj39
iEAduROqDi8XnnkCGb2RP5ATEII0YAauROjGAlV2oQKBgD4yneSwi1i8gfd4fEUS
tuRB4AkX6EHw6E9Zjj/gwttVt1vYM8dbam5aZPlP602yRRUrt0T101zE+s0SBQZh
9IUctJHxGO/5cufDZvovw2pXKlZkcpDxwPoKiUQZxiPBXf8YfKHUXz0gSc6QHAzu
XinNZUVoxqiVkt4smBecyfGS
-----END PRIVATE KEY-----`

func ap2Total(contents map[string]any) any {
	return contents["payment_request"].(map[string]any)["details"].(map[string]any)["total"].(map[string]any)["amount"].(map[string]any)["value"]
}

func TestAP2(t *testing.T) {
	tr := AP2Transport(nil)
	if tr["enabled"] != true || tr["version"] != "0.2.0" {
		t.Fatalf("transport: %v", tr)
	}
	if s, _ := tr["extension"].(string); s == "" {
		t.Fatal("transport: no extension uri")
	}

	golden := map[string]any{"z": "a/b", "currency": "GBP", "n": 10.0, "items": []any{map[string]any{"value": 9.99, "label": "Mug"}}, "ok": true}
	if got := string(ap2Canonical(golden)); got != `{"currency":"GBP","items":[{"label":"Mug","value":9.99}],"n":10,"ok":true,"z":"a/b"}` {
		t.Fatalf("JCS canonical cross-SDK: %s", got)
	}

	intent := AP2IntentMandate("a red basketball shoe", AP2IntentOptions{Skus: []string{"SHOE-1"}, Now: 1000})
	if intent["natural_language_description"] != "a red basketball shoe" || intent["intent_expiry"] == "" {
		t.Fatalf("intent: %v", intent)
	}

	contents := AP2CartContents([]AP2LineItem{{Label: "Mug", UnitAmount: 9.99, Quantity: 3}}, "GBP", "Test Store", AP2Options{Now: 1000})
	if got := ap2Total(contents); got != 29.97 {
		t.Fatalf("cart total = %v, want 29.97", got)
	}

	mandate, err := AP2CartMandate(contents, ap2TestKey, AP2Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !AP2VerifyCartMandate(mandate, ap2TestKey) {
		t.Fatal("valid cart mandate did not verify")
	}

	// Tamper: change the total; verification must fail.
	contents["payment_request"].(map[string]any)["details"].(map[string]any)["total"].(map[string]any)["amount"].(map[string]any)["value"] = 0.01
	if AP2VerifyCartMandate(mandate, ap2TestKey) {
		t.Fatal("tampered cart mandate verified")
	}

	jwks, err := AP2JWKS(ap2TestKey, "")
	if err != nil {
		t.Fatal(err)
	}
	k0 := jwks["keys"].([]any)[0].(map[string]any)
	if k0["kty"] != "RSA" || k0["alg"] != "RS256" || k0["n"] == "" {
		t.Fatalf("jwks: %v", k0)
	}

	pd := AP2PaymentDetails(map[string]any{"payment_mandate_contents": map[string]any{
		"payment_mandate_id":    "pm_1",
		"payment_details_id":    "pr_x",
		"payment_details_total": map[string]any{"label": "Total", "amount": AP2Amount(29.97, "GBP")},
		"payment_response":      map[string]any{"method_name": "card", "payer_email": "a@b.com"},
	}})
	if pd["payment_details_id"] != "pr_x" || pd["method"] != "card" || pd["payer_email"] != "a@b.com" {
		t.Fatalf("payment details: %v", pd)
	}
}
