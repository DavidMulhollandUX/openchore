package api

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/mealie"
)

// MealsHandler serves the Meals screen from a Mealie server. Mealie stays the
// place recipes are written; OpenChore shows the plan, the recipes and the
// shopping list on the shared tablet, and lets a signed-in parent change the
// plan.
//
// Meals is off unless MEALIE_URL and MEALIE_API_TOKEN are set. The token
// stays on the server. MEALIE_PUBLIC_URL is where browsers reach Mealie (the
// "Add or edit recipes in Mealie" link); it defaults to MEALIE_URL.
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
		items = append(items, shoppingItemView{ID: it.ID, Label: it.Label(), Checked: it.Checked})
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

// recipesPerPage is how many recipe cards one page of the Recipes tab holds.
const recipesPerPage = 24

// recipeImageFiles are the photo sizes Mealie keeps for a recipe.
var recipeImageFiles = map[string]bool{
	"original.webp": true, "min-original.webp": true, "tiny-original.webp": true,
}

var recipeIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]{1,64}$`)

type recipeCard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	TotalTime   string `json:"total_time,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
}

// recipeImageURL is where the browser fetches a recipe photo: through
// OpenChore, so it works wherever OpenChore does. The cache key changes
// when the photo does.
func recipeImageURL(rc mealie.Recipe, file string) string {
	if rc.Image == "" || !recipeIDPattern.MatchString(rc.ID) {
		return ""
	}
	return "/api/meals/recipe-images/" + rc.ID + "/" + file + "?v=" + url.QueryEscape(rc.Image)
}

func cardFrom(rc mealie.Recipe) recipeCard {
	return recipeCard{
		ID: rc.ID, Name: rc.Name, Slug: rc.Slug, TotalTime: rc.TotalTime,
		Description: strings.TrimSpace(rc.Description),
		Image:       recipeImageURL(rc, "min-original.webp"),
	}
}

// Cookbook lists the household's recipes for the Recipes tab, A to Z,
// ?search= to narrow, ?page= from 1. Anyone at the tablet can read them.
func (h *MealsHandler) Cookbook(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	c := h.client(w)
	if c == nil {
		return
	}
	found, err := c.ListRecipes(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")), page, recipesPerPage)
	if err != nil {
		mealieFailed(w, err)
		return
	}
	items := make([]recipeCard, 0, len(found.Items))
	for _, rc := range found.Items {
		items = append(items, cardFrom(rc))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": found.Total, "page": page, "per_page": recipesPerPage,
	})
}

type ingredientView struct {
	Section string `json:"section,omitempty"`
	Text    string `json:"text,omitempty"`
}

type stepView struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

// RecipeDetail returns one recipe for the recipe page.
func (h *MealsHandler) RecipeDetail(w http.ResponseWriter, r *http.Request) {
	c := h.client(w)
	if c == nil {
		return
	}
	rc, err := c.GetRecipe(r.Context(), urlParam(r, "slug"))
	if err != nil {
		mealieFailed(w, err)
		return
	}
	ingredients := make([]ingredientView, 0, len(rc.Ingredients))
	for _, in := range rc.Ingredients {
		text := strings.TrimSpace(in.Display)
		if text == "" {
			text = strings.TrimSpace(in.Note)
		}
		if title := strings.TrimSpace(in.Title); title != "" {
			ingredients = append(ingredients, ingredientView{Section: title})
		}
		if text != "" {
			ingredients = append(ingredients, ingredientView{Text: text})
		}
	}
	steps := make([]stepView, 0, len(rc.Steps))
	for _, st := range rc.Steps {
		if text := strings.TrimSpace(st.Text); text != "" {
			steps = append(steps, stepView{Title: strings.TrimSpace(st.Title), Text: text})
		}
	}
	notes := make([]stepView, 0, len(rc.Notes))
	for _, n := range rc.Notes {
		if text := strings.TrimSpace(n.Text); text != "" {
			notes = append(notes, stepView{Title: strings.TrimSpace(n.Title), Text: text})
		}
	}
	cookTime := rc.PerformTime
	if cookTime == "" {
		cookTime = rc.CookTime
	}
	var servings string
	if rc.Servings > 0 {
		servings = strconv.FormatFloat(rc.Servings, 'f', -1, 64)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": rc.ID, "name": rc.Name, "slug": rc.Slug,
		"description": strings.TrimSpace(rc.Description),
		"image":       recipeImageURL(rc.Recipe, "original.webp"),
		"total_time":  rc.TotalTime, "prep_time": rc.PrepTime, "cook_time": cookTime,
		"yield": strings.TrimSpace(rc.Yield), "servings": servings,
		"ingredients": ingredients, "steps": steps, "notes": notes,
	})
}

// RecipeImage passes a recipe photo on from Mealie.
func (h *MealsHandler) RecipeImage(w http.ResponseWriter, r *http.Request) {
	id, file := urlParam(r, "id"), urlParam(r, "file")
	if !recipeIDPattern.MatchString(id) || !recipeImageFiles[file] {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	c := h.client(w)
	if c == nil {
		return
	}
	body, contentType, err := c.RecipeImage(r.Context(), id, file)
	if err != nil {
		mealieFailed(w, err)
		return
	}
	defer body.Close()
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/webp"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, body)
}
