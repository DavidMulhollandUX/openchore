// Package mealie talks to a Mealie server (https://mealie.io) for the Meals
// screen: the household's meal plan, its recipes and its shopping list.
// Written against Mealie v3.29's API; Mealie answers in camelCase.
package mealie

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client calls one Mealie server with a long-lived API token.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a client. baseURL is the server root, e.g. "http://mealie:9000".
func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Recipe is the part of a Mealie recipe summary the Meals screen shows.
type Recipe struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	TotalTime string `json:"totalTime"`
}

// PlanEntry is one meal on one day.
type PlanEntry struct {
	ID        int64   `json:"id"`
	Date      string  `json:"date"`
	EntryType string  `json:"entryType"`
	Title     string  `json:"title"`
	Text      string  `json:"text"`
	RecipeID  *string `json:"recipeId"`
	Recipe    *Recipe `json:"recipe"`
}

// NewPlanEntry is what Create sends. Mealie needs a recipe or a title.
type NewPlanEntry struct {
	Date      string `json:"date"`
	EntryType string `json:"entryType"`
	Title     string `json:"title"`
	RecipeID  string `json:"recipeId,omitempty"`
}

// ShoppingItem is one line on a shopping list.
type ShoppingItem struct {
	ID       string          `json:"id"`
	Display  string          `json:"display"`
	Note     string          `json:"note"`
	Quantity float64         `json:"quantity"`
	Food     json.RawMessage `json:"food"`
	Unit     json.RawMessage `json:"unit"`
	Checked  bool            `json:"checked"`
	Position int             `json:"position"`
}

// Label is how Mealie's own list shows the item. Display always leads with
// the quantity ("1 Milk"); for a plain note item with no food or unit,
// Mealie's list shows just the note unless the quantity is more than one.
func (i ShoppingItem) Label() string {
	isSet := func(raw json.RawMessage) bool { return len(raw) > 0 && string(raw) != "null" }
	if isSet(i.Food) || isSet(i.Unit) || strings.TrimSpace(i.Note) == "" {
		return strings.TrimSpace(i.Display)
	}
	note := strings.TrimSpace(i.Note)
	if i.Quantity > 1 {
		return strconv.FormatFloat(i.Quantity, 'f', -1, 64) + " " + note
	}
	return note
}

// ShoppingList is a list and its items.
type ShoppingList struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Items []ShoppingItem `json:"listItems"`
}

// Error is a non-2xx answer from Mealie.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("mealie: HTTP %d: %s", e.Status, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mealie: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &Error{Status: resp.StatusCode, Body: strings.TrimSpace(string(msg))}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Plan lists the meals planned from start to end inclusive (YYYY-MM-DD).
func (c *Client) Plan(ctx context.Context, start, end string) ([]PlanEntry, error) {
	q := url.Values{}
	q.Set("start_date", start)
	q.Set("end_date", end)
	q.Set("perPage", "-1")
	q.Set("orderBy", "date")
	q.Set("orderDirection", "asc")
	var page struct {
		Items []PlanEntry `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/households/mealplans?"+q.Encode(), nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// CreatePlanEntry adds a meal to the plan.
func (c *Client) CreatePlanEntry(ctx context.Context, e NewPlanEntry) (PlanEntry, error) {
	var out PlanEntry
	err := c.do(ctx, http.MethodPost, "/api/households/mealplans", e, &out)
	return out, err
}

// DeletePlanEntry removes a meal from the plan.
func (c *Client) DeletePlanEntry(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/households/mealplans/%d", id), nil, nil)
}

// SearchRecipes finds up to 20 recipes by name.
func (c *Client) SearchRecipes(ctx context.Context, search string) ([]Recipe, error) {
	q := url.Values{}
	q.Set("search", search)
	q.Set("perPage", "20")
	var page struct {
		Items []Recipe `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/recipes?"+q.Encode(), nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// FirstShoppingList returns the household's first shopping list with its
// items, or nil when it has none.
func (c *Client) FirstShoppingList(ctx context.Context) (*ShoppingList, error) {
	var page struct {
		Items []ShoppingList `json:"items"`
	}
	q := url.Values{}
	q.Set("perPage", "1")
	if err := c.do(ctx, http.MethodGet, "/api/households/shopping/lists?"+q.Encode(), nil, &page); err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, nil
	}
	var list ShoppingList
	if err := c.do(ctx, http.MethodGet, "/api/households/shopping/lists/"+url.PathEscape(page.Items[0].ID), nil, &list); err != nil {
		return nil, err
	}
	sort.SliceStable(list.Items, func(a, b int) bool { return list.Items[a].Position < list.Items[b].Position })
	return &list, nil
}

// SetItemChecked ticks or unticks a shopping list item. Mealie's update
// takes the whole item, so it is read, changed and sent back as Mealie
// returned it.
func (c *Client) SetItemChecked(ctx context.Context, id string, checked bool) error {
	path := "/api/households/shopping/items/" + url.PathEscape(id)
	var item map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &item); err != nil {
		return err
	}
	item["checked"] = checked
	return c.do(ctx, http.MethodPut, path, item, nil)
}
