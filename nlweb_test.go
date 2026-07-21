package ai2web

import "testing"

func TestNlweb(t *testing.T) {
	tr := NlwebTransport(nil)
	if tr["enabled"] != true || tr["version"] != "0.55" || tr["ask"] == "" {
		t.Fatalf("transport: %v", tr)
	}

	resp := NlwebAskResponse("red shoes", []map[string]any{
		{"url": "https://s.example/1", "name": "Red Shoe", "description": "A red running shoe", "score": 90},
		{"url": "https://s.example/2", "title": "Crimson Sneaker"},
	}, NlwebResponseOptions{Site: "store"})

	if resp["query"] != "red shoes" {
		t.Fatalf("query: %v", resp["query"])
	}
	results, _ := resp["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results len: %d", len(results))
	}
	r0 := results[0].(map[string]any)
	if r0["@type"] != "Item" || r0["name"] != "Red Shoe" || r0["score"] != 90 || r0["site"] != "store" {
		t.Fatalf("r0: %v", r0)
	}
	r1 := results[1].(map[string]any)
	if r1["name"] != "Crimson Sneaker" {
		t.Fatalf("r1 name: %v", r1["name"])
	}
	so := r1["schema_object"].(map[string]any)
	if so["@type"] != "Thing" || so["name"] != "Crimson Sneaker" {
		t.Fatalf("schema_object: %v", so)
	}
}
