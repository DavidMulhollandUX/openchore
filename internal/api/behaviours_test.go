package api_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestBehavioursCRUDAndApply(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Kids can't manage behaviours or apply them to themselves.
	env.expectStatus(t, "POST", "/api/behaviours", map[string]any{
		"name": "Made bed", "points": 5,
	}, childHeaders(kidID), http.StatusForbidden)

	// Validation
	env.expectStatus(t, "POST", "/api/behaviours", map[string]any{
		"name": "  ", "points": 5,
	}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/behaviours", map[string]any{
		"name": "Nothing", "points": 0,
	}, adminHeaders(), http.StatusBadRequest)

	var good, bad map[string]any
	resp := env.expectStatus(t, "POST", "/api/behaviours", map[string]any{
		"name": "Kind to sibling", "points": 5,
	}, adminHeaders(), http.StatusCreated)
	decodeBody(t, resp, &good)
	resp = env.expectStatus(t, "POST", "/api/behaviours", map[string]any{
		"name": "Hitting", "points": -10,
	}, adminHeaders(), http.StatusCreated)
	decodeBody(t, resp, &bad)
	goodID := int64(good["id"].(float64))
	badID := int64(bad["id"].(float64))

	// Good behaviours list first.
	resp = env.expectStatus(t, "GET", "/api/behaviours", nil, adminHeaders(), http.StatusOK)
	var list []map[string]any
	decodeBody(t, resp, &list)
	if len(list) != 2 || list[0]["name"] != "Kind to sibling" || list[1]["name"] != "Hitting" {
		t.Fatalf("unexpected list: %v", list)
	}

	// Apply both; the kid ends at 5 - 10 = -5.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/behaviours/%d/apply", goodID), map[string]any{
		"user_id": kidID,
	}, adminHeaders(), http.StatusCreated)
	resp = env.expectStatus(t, "POST", fmt.Sprintf("/api/behaviours/%d/apply", badID), map[string]any{
		"user_id": kidID, "note": "at dinner",
	}, adminHeaders(), http.StatusCreated)
	var tx map[string]any
	decodeBody(t, resp, &tx)
	if tx["reason"] != "behaviour" || tx["amount"].(float64) != -10 || tx["note"] != "Hitting — at dinner" {
		t.Fatalf("unexpected transaction: %v", tx)
	}

	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != -5 {
		t.Fatalf("expected -5, got %v", pts["balance"])
	}

	// Renaming or deleting a behaviour leaves the ledger untouched.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/behaviours/%d", badID), map[string]any{
		"name": "Hitting or kicking", "points": -15,
	}, adminHeaders(), http.StatusOK)
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/behaviours/%d", goodID), nil, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != -5 {
		t.Fatalf("expected -5 after edits, got %v", pts["balance"])
	}

	// Unknown behaviour or person
	env.expectStatus(t, "POST", fmt.Sprintf("/api/behaviours/%d/apply", goodID), map[string]any{
		"user_id": kidID,
	}, adminHeaders(), http.StatusNotFound)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/behaviours/%d/apply", badID), map[string]any{
		"user_id": 9999,
	}, adminHeaders(), http.StatusNotFound)
}
