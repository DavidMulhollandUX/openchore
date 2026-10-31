package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/mealie"
)

// MealsHandler serves the Meals screen from a Mealie server. Mealie stays the
// place recipes are written; OpenChore shows the plan and the shopping list
// on the shared tablet, and lets a signed-in parent change the plan.
//
// Meals is off unless MEALIE_URL and MEALIE_API_TOKEN are set. The token
// stays on the server. MEALIE_PUBLIC_URL is where browsers reach Mealie (the
// "Open in Mealie" link); it defaults to MEALIE_URL.
type MealsHandler struct{}

func NewMealsHandler() *MealsHandler {
	return &MealsHandler{}
}

// maxPlanDays caps how many days one plan request may cover.
const maxPlanDays = 31

// mealTypes are Mealie's plan entry types.
var mealTypes = map[string]bool{
	"breakfast": true, "lunch": true, "dinner": true, "side": true,
	"snack": true, "drink": true, "dessert": true,
}

func mealieClient() *mealie.Client {
	base, token := os.Getenv("MEALIE_URL"), os.Getenv("MEALIE_API_TOKEN")
	if base == "" || token == "" {
		return nil
	}
	return mealie.New(base, token)
}

func mealiePublicURL() string {
	if u := os.Getenv("MEALIE_PUBLIC_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return strings.TrimRight(os.Getenv("MEALIE_URL"), "/")
}

// client returns the Mealie client, or writes a 503 and returns nil.
func (h *MealsHandler) client(w http.ResponseWriter) *mealie.Client {
	c := mealieClient()
	if c == nil {
		writeError(w, http.StatusServiceUnavailable, "meals are not set up")
	}
	return c
}

// mealieFailed reports a Mealie failure without passing Mealie's body on.
func mealieFailed(w http.ResponseWriter, err error) {
	log.Printf("meals: %v", err)
	var me *mealie.Error
	if errors.As(err, &me) && me.Status == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "not found in Mealie")
		return
	}
	writeError(w, http.StatusBadGateway, "Mealie didn't answer")
}

// Status says whether Meals is on and where Mealie lives.
func (h *MealsHandler) Status(w http.ResponseWriter, r *http.Request) {
	enabled := mealieClient() != nil
	resp := map[string]any{"enabled": enabled}
	if enabled {
		resp["mealie_url"] = mealiePublicURL()
	}
	writeJSON(w, http.StatusOK, resp)
}

type mealView struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Meal       string `json:"meal"`
	Title      string `json:"title"`
	Note       string `json:"note,omitempty"`
	RecipeSlug string `json:"recipe_slug,omitempty"`
	TotalTime  string `json:"total_time,omitempty"`
}

func mealFrom(e mealie.PlanEntry) mealView {
	v := mealView{ID: e.ID, Date: e.Date, Meal: e.EntryType, Title: e.Title, Note: e.Text}
	if e.Recipe != nil {
		if v.Title == "" {
			v.Title = e.Recipe.Name
		}
		v.RecipeSlug = e.Recipe.Slug
		v.TotalTime = e.Recipe.TotalTime
	}
	return v
}

func parseDay(s string) (time.Time, bool) {
	d, err := time.Parse(time.DateOnly, s)
	return d, err == nil
}

// Plan lists the meals from ?start= to ?end= (YYYY-MM-DD, inclusive).
func (h *MealsHandler) Plan(w http.ResponseWriter, r *http.Request) {
	start, end := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	s, okS := parseDay(start)
	e, okE := parseDay(end)
	if !okS || !okE || e.Before(s) {
		writeError(w, http.StatusBadRequest, "start and end must be dates (YYYY-MM-DD), start first")
		return
	}
	if e.Sub(s) > maxPlanDays*24*time.Hour {
		writeError(w, http.StatusBadRequest, "ask for 31 days or fewer")
		return
	}
	c := h.client(w)
	if c == nil {
		return
	}
	entries, err := c.Plan(r.Context(), start, end)
	if err != nil {
		mealieFailed(w, err)
		return
	}
	meals := make([]mealView, 0, len(entries))
	for _, entry := range entries {
		meals = append(meals, mealFrom(entry))
	}
	writeJSON(w, http.StatusOK, meals)
}

// AddMeal plans a meal (parents only): a recipe, or just a title.
func (h *MealsHandler) AddMeal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date     string `json:"date"`
		Meal     string `json:"meal"`
		Title    string `json:"title"`
		RecipeID string `json:"recipe_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if _, ok := parseDay(req.Date); !ok {
		writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	if !mealTypes[req.Meal] {
		writeError(w, http.StatusBadRequest, "unknown meal")
		return
	}
	if req.Title == "" && req.RecipeID == "" {
		writeError(w, http.StatusBadRequest, "pick a recipe or type a title")
		return
	}
	c := h.client(w)
	if c == nil {
		return
	}
	entry, err := c.CreatePlanEntry(r.Context(), mealie.NewPlanEntry{
		Date: req.Date, EntryType: req.Meal, Title: req.Title, RecipeID: req.RecipeID,
	})
	if err != nil {
		mealieFailed(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mealFrom(entry))
}

// RemoveMeal takes a meal off the plan (parents only).
func (h *MealsHandler) RemoveMeal(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	c := h.client(w)
	if c == nil {
		return
	}
	if err := c.DeletePlanEntry(r.Context(), id); err != nil {
		mealieFailed(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Recipes searches Mealie's recipes by name (parents only, for planning).
func (h *MealsHandler) Recipes(w http.ResponseWriter, r *http.Request) {
	c := h.client(w)
	if c == nil {
		return
	}
	recipes, err := c.SearchRecipes(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")))
	if err != nil {
		mealieFailed(w, err)
		return
	}
	type recipeView struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	out := make([]recipeView, 0, len(recipes))
	for _, rc := range recipes {
		out = append(out, recipeView{ID: rc.ID, Name: rc.Name, Slug: rc.Slug})
	}
	writeJSON(w, http.StatusOK, out)
}

type shoppingItemView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Checked bool   `json:"checked"`
}

// Shopping returns the household's shopping list, or null when it has none.
func (h *MealsHandler) Shopping(w http.ResponseWriter, r *http.Request) {
	c := h.client(w)
	if c == nil {
		return
	}
	list, err := c.FirstShoppingList(r.Context())
	if err != nil {
		mealieFailed(w, err)
		return
	}
	if list == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	items := make([]shoppingItemView, 0, len(list.Items))
	for _, it := range list.Items {
		label := strings.TrimSpace(it.Display)
		if label == "" {
			label = strings.TrimSpace(it.Note)
		}
		items = append(items, shoppingItemView{ID: it.ID, Label: label, Checked: it.Checked})
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": list.ID, "name": list.Name, "items": items})
}

// CheckItem ticks or unticks a shopping list item. Anyone at the tablet can.
func (h *MealsHandler) CheckItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Checked bool `json:"checked"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c := h.client(w)
	if c == nil {
		return
	}
	if err := c.SetItemChecked(r.Context(), urlParam(r, "id"), req.Checked); err != nil {
		mealieFailed(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
