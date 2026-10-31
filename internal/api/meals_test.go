package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeMealie answers the handful of Mealie v3 endpoints the Meals screen
// uses, in Mealie's camelCase, and records what it was sent.
type fakeMealie struct {
	mu      sync.Mutex
	server  *httptest.Server
	auth    string
	plans   []map[string]any
	created []map[string]any
	deleted []string
	items   map[string]map[string]any
	query   string
}

func newFakeMealie(t *testing.T) *fakeMealie {
	t.Helper()
	f := &fakeMealie{
		plans: []map[string]any{
			{"id": 7, "date": "2026-11-02", "entryType": "dinner", "title": "", "text": "",
				"recipe": map[string]any{"id": "r1", "name": "Tacos", "slug": "tacos", "totalTime": "30 min"}},
			{"id": 8, "date": "2026-11-03", "entryType": "lunch", "title": "Leftovers", "text": "", "recipe": nil},
		},
		items: map[string]map[string]any{
			"i1": {"id": "i1", "shoppingListId": "l1", "display": "2 avocados", "note": "", "checked": false, "position": 0, "quantity": 2},
			"i2": {"id": "i2", "shoppingListId": "l1", "display": "1 Milk", "note": "Milk", "checked": true, "position": 1, "quantity": 1, "food": nil, "unit": nil},
			"i3": {"id": "i3", "shoppingListId": "l1", "display": "3 Eggs", "note": "Eggs", "checked": false, "position": 2, "quantity": 3},
			"i0": {"id": "i0", "shoppingListId": "l1", "display": "500 g Flour", "note": "", "checked": false, "position": -1, "quantity": 500,
				"food": map[string]any{"name": "Flour"}, "unit": map[string]any{"name": "g"}},
		},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	t.Setenv("MEALIE_URL", f.server.URL)
	t.Setenv("MEALIE_API_TOKEN", "test-token")
	t.Setenv("MEALIE_PUBLIC_URL", "https://meals.example.test/")
	return f
}

func (f *fakeMealie) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/api/households/mealplans":
		f.query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"items": f.plans})
	case r.Method == http.MethodPost && path == "/api/households/mealplans":
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.created = append(f.created, body)
		body["id"] = 9
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/households/mealplans/"):
		id := strings.TrimPrefix(path, "/api/households/mealplans/")
		if id == "404" {
			http.Error(w, `{"detail":"nope"}`, http.StatusNotFound)
			return
		}
		f.deleted = append(f.deleted, id)
		w.Write([]byte(`{}`))
	case r.Method == http.MethodGet && path == "/api/recipes":
		f.query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"total": 1, "items": []map[string]any{
			{"id": "0b6e5c1a-0000-4000-8000-000000000001", "name": "Tacos", "slug": "tacos", "totalTime": "30 min",
				"description": " Crunchy. ", "image": "ab12"},
		}})
	case r.Method == http.MethodGet && path == "/api/recipes/tacos":
		json.NewEncoder(w).Encode(map[string]any{
			"id": "0b6e5c1a-0000-4000-8000-000000000001", "name": "Tacos", "slug": "tacos", "image": nil,
			"totalTime": "30 min", "prepTime": "10 min", "performTime": nil, "cookTime": "20 min",
			"recipeYield": "4 tacos", "recipeServings": 2.0, "description": nil,
			"recipeIngredient": []map[string]any{
				{"title": "Filling", "display": "500 g mince", "note": ""},
				{"title": nil, "display": "", "note": "Salsa"},
				{"title": nil, "display": "", "note": ""},
			},
			"recipeInstructions": []map[string]any{
				{"title": "", "text": "Brown the **mince**."},
				{"title": "Serve", "text": "Fill the shells."},
				{"title": "", "text": "  "},
			},
			"notes": []map[string]any{{"title": "Tip", "text": "Warm the shells."}},
		})
	case r.Method == http.MethodGet && path == "/api/recipes/missing":
		http.Error(w, `{"detail":"nope"}`, http.StatusNotFound)
	case r.Method == http.MethodGet && path == "/api/media/recipes/0b6e5c1a-0000-4000-8000-000000000001/images/min-original.webp":
		w.Header().Set("Content-Type", "image/webp")
		w.Write([]byte("RIFFwebp"))
	case r.Method == http.MethodGet && path == "/api/households/shopping/lists":
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": "l1", "name": "Groceries"}}})
	case r.Method == http.MethodGet && path == "/api/households/shopping/lists/l1":
		list := []map[string]any{f.items["i1"], f.items["i2"], f.items["i3"], f.items["i0"]}
		json.NewEncoder(w).Encode(map[string]any{"id": "l1", "name": "Groceries", "listItems": list})
	case strings.HasPrefix(path, "/api/households/shopping/items/"):
		item, ok := f.items[strings.TrimPrefix(path, "/api/households/shopping/items/")]
		if !ok {
			http.Error(w, `{"detail":"nope"}`, http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPut {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			for k, v := range body {
				item[k] = v
			}
		}
		json.NewEncoder(w).Encode(item)
	default:
		http.Error(w, "unexpected "+r.Method+" "+path, http.StatusTeapot)
	}
}

func TestMealsOffWithoutMealie(t *testing.T) {
	env := setupTest(t)
	t.Setenv("MEALIE_URL", "")
	t.Setenv("MEALIE_API_TOKEN", "")

	var status map[string]any
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/status", nil, nil, http.StatusOK), &status)
	if status["enabled"] != false {
		t.Fatalf("expected meals off, got %v", status)
	}
	env.expectStatus(t, "GET", "/api/meals/plan?start=2026-11-02&end=2026-11-08", nil, nil, http.StatusServiceUnavailable)
	env.expectStatus(t, "GET", "/api/meals/shopping", nil, nil, http.StatusServiceUnavailable)
}

func TestMealsStatusAndPlan(t *testing.T) {
	env := setupTest(t)
	fake := newFakeMealie(t)

	var status map[string]any
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/status", nil, nil, http.StatusOK), &status)
	if status["enabled"] != true || status["mealie_url"] != "https://meals.example.test" {
		t.Fatalf("unexpected status %v", status)
	}

	// No sign-in needed to read the plan.
	var meals []map[string]any
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/plan?start=2026-11-02&end=2026-11-08", nil, nil, http.StatusOK), &meals)
	if len(meals) != 2 {
		t.Fatalf("expected 2 meals, got %v", meals)
	}
	if meals[0]["title"] != "Tacos" || meals[0]["meal"] != "dinner" || meals[0]["recipe_slug"] != "tacos" || meals[0]["total_time"] != "30 min" {
		t.Errorf("recipe meal not mapped: %v", meals[0])
	}
	if meals[1]["title"] != "Leftovers" {
		t.Errorf("titled meal not mapped: %v", meals[1])
	}
	if fake.auth != "Bearer test-token" {
		t.Errorf("Mealie got auth %q", fake.auth)
	}
	if !strings.Contains(fake.query, "start_date=2026-11-02") || !strings.Contains(fake.query, "end_date=2026-11-08") {
		t.Errorf("Mealie got query %q", fake.query)
	}
}

func TestMealsPlanValidatesDates(t *testing.T) {
	env := setupTest(t)
	newFakeMealie(t)
	env.expectStatus(t, "GET", "/api/meals/plan?start=nope&end=2026-11-08", nil, nil, http.StatusBadRequest)
	env.expectStatus(t, "GET", "/api/meals/plan?start=2026-11-08&end=2026-11-02", nil, nil, http.StatusBadRequest)
	env.expectStatus(t, "GET", "/api/meals/plan?start=2026-01-01&end=2026-03-01", nil, nil, http.StatusBadRequest)
}

func TestMealsChangingThePlanNeedsAParent(t *testing.T) {
	env := setupTest(t)
	fake := newFakeMealie(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Lily")
	add := map[string]any{"date": "2026-11-04", "meal": "dinner", "recipe_id": "r1"}

	env.expectStatus(t, "POST", "/api/meals/plan", add, nil, http.StatusUnauthorized)
	env.expectStatus(t, "POST", "/api/meals/plan", add, childHeaders(kid), http.StatusForbidden)
	env.expectStatus(t, "DELETE", "/api/meals/plan/7", nil, childHeaders(kid), http.StatusForbidden)
	env.expectStatus(t, "GET", "/api/meals/recipes?search=taco", nil, childHeaders(kid), http.StatusForbidden)
	if len(fake.created) != 0 || len(fake.deleted) != 0 {
		t.Fatalf("Mealie was changed without a parent: %v %v", fake.created, fake.deleted)
	}

	env.expectStatus(t, "POST", "/api/meals/plan", add, adminHeaders(), http.StatusCreated)
	if len(fake.created) != 1 || fake.created[0]["recipeId"] != "r1" || fake.created[0]["entryType"] != "dinner" || fake.created[0]["date"] != "2026-11-04" {
		t.Fatalf("Mealie got %v", fake.created)
	}
	env.expectStatus(t, "DELETE", "/api/meals/plan/7", nil, adminHeaders(), http.StatusNoContent)
	if len(fake.deleted) != 1 || fake.deleted[0] != "7" {
		t.Fatalf("Mealie deleted %v", fake.deleted)
	}
	env.expectStatus(t, "DELETE", "/api/meals/plan/404", nil, adminHeaders(), http.StatusNotFound)

	var recipes []map[string]any
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/recipes?search=taco", nil, adminHeaders(), http.StatusOK), &recipes)
	if len(recipes) != 1 || recipes[0]["name"] != "Tacos" {
		t.Fatalf("unexpected recipes %v", recipes)
	}
}

func TestMealsAddValidation(t *testing.T) {
	env := setupTest(t)
	newFakeMealie(t)
	env.createAdmin(t)
	env.expectStatus(t, "POST", "/api/meals/plan", map[string]any{"date": "soon", "meal": "dinner", "title": "Soup"}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/meals/plan", map[string]any{"date": "2026-11-04", "meal": "brunch", "title": "Soup"}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/meals/plan", map[string]any{"date": "2026-11-04", "meal": "dinner", "title": "  "}, adminHeaders(), http.StatusBadRequest)
}

func TestMealsShoppingList(t *testing.T) {
	env := setupTest(t)
	fake := newFakeMealie(t)

	var list struct {
		Name  string `json:"name"`
		Items []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Checked bool   `json:"checked"`
		} `json:"items"`
	}
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/shopping", nil, nil, http.StatusOK), &list)
	if list.Name != "Groceries" || len(list.Items) != 4 {
		t.Fatalf("unexpected list %+v", list)
	}
	// In Mealie's position order, labelled the way Mealie's own list shows them:
	// a plain note drops a quantity of one, a food with a unit keeps both.
	var labels []string
	for _, it := range list.Items {
		labels = append(labels, it.Label)
	}
	if got := strings.Join(labels, "|"); got != "500 g Flour|2 avocados|Milk|3 Eggs" {
		t.Errorf("labels = %q", got)
	}
	if !list.Items[2].Checked {
		t.Errorf("Milk should be checked: %+v", list.Items[2])
	}

	// Anyone at the tablet can tick an item; the rest of the item goes back unchanged.
	env.expectStatus(t, "PUT", "/api/meals/shopping/items/i1", map[string]any{"checked": true}, nil, http.StatusNoContent)
	fake.mu.Lock()
	item := fake.items["i1"]
	fake.mu.Unlock()
	if item["checked"] != true || item["display"] != "2 avocados" || item["shoppingListId"] != "l1" {
		t.Fatalf("item after tick: %v", item)
	}
	env.expectStatus(t, "PUT", "/api/meals/shopping/items/missing", map[string]any{"checked": true}, nil, http.StatusNotFound)
}

func TestMealsCookbook(t *testing.T) {
	env := setupTest(t)
	fake := newFakeMealie(t)

	// Anyone at the tablet can browse recipes, A to Z.
	var page struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
		Page  int              `json:"page"`
	}
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/cookbook?search=taco&page=2", nil, nil, http.StatusOK), &page)
	if page.Total != 1 || page.Page != 2 || len(page.Items) != 1 {
		t.Fatalf("unexpected page %+v", page)
	}
	card := page.Items[0]
	if card["name"] != "Tacos" || card["description"] != "Crunchy." || card["total_time"] != "30 min" ||
		card["image"] != "/api/meals/recipe-images/0b6e5c1a-0000-4000-8000-000000000001/min-original.webp?v=ab12" {
		t.Errorf("card not mapped: %v", card)
	}
	for _, want := range []string{"search=taco", "page=2", "orderBy=name"} {
		if !strings.Contains(fake.query, want) {
			t.Errorf("Mealie got query %q, want %s", fake.query, want)
		}
	}
	env.expectStatus(t, "GET", "/api/meals/cookbook?page=-3", nil, nil, http.StatusOK)
}

func TestMealsRecipePage(t *testing.T) {
	env := setupTest(t)
	newFakeMealie(t)

	var rc struct {
		Name        string              `json:"name"`
		Image       string              `json:"image"`
		CookTime    string              `json:"cook_time"`
		Servings    string              `json:"servings"`
		Ingredients []map[string]string `json:"ingredients"`
		Steps       []map[string]string `json:"steps"`
		Notes       []map[string]string `json:"notes"`
	}
	decodeBody(t, env.expectStatus(t, "GET", "/api/meals/cookbook/tacos", nil, nil, http.StatusOK), &rc)
	if rc.Name != "Tacos" || rc.Image != "" || rc.CookTime != "20 min" || rc.Servings != "2" {
		t.Errorf("recipe not mapped: %+v", rc)
	}
	// A section title comes before its line; empty lines and steps are dropped.
	if len(rc.Ingredients) != 3 || rc.Ingredients[0]["section"] != "Filling" ||
		rc.Ingredients[1]["text"] != "500 g mince" || rc.Ingredients[2]["text"] != "Salsa" {
		t.Errorf("ingredients = %v", rc.Ingredients)
	}
	if len(rc.Steps) != 2 || rc.Steps[0]["text"] != "Brown the **mince**." || rc.Steps[1]["title"] != "Serve" {
		t.Errorf("steps = %v", rc.Steps)
	}
	if len(rc.Notes) != 1 || rc.Notes[0]["title"] != "Tip" {
		t.Errorf("notes = %v", rc.Notes)
	}
	env.expectStatus(t, "GET", "/api/meals/cookbook/missing", nil, nil, http.StatusNotFound)
}

func TestMealsRecipeImage(t *testing.T) {
	env := setupTest(t)
	newFakeMealie(t)

	resp := env.expectStatus(t, "GET", "/api/meals/recipe-images/0b6e5c1a-0000-4000-8000-000000000001/min-original.webp", nil, nil, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
		t.Errorf("content type %q", ct)
	}
	// Only Mealie's photo files, for a recipe id.
	env.expectStatus(t, "GET", "/api/meals/recipe-images/0b6e5c1a-0000-4000-8000-000000000001/secrets.txt", nil, nil, http.StatusNotFound)
	env.expectStatus(t, "GET", "/api/meals/recipe-images/..%2Fusers/original.webp", nil, nil, http.StatusNotFound)
}
