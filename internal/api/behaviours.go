package api

import (
	"net/http"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// BehaviourHandler manages the parent-defined list of behaviours and applies
// them to a person's points in one tap.
type BehaviourHandler struct {
	store *store.Store
}

func NewBehaviourHandler(s *store.Store) *BehaviourHandler {
	return &BehaviourHandler{store: s}
}

type behaviourRequest struct {
	Name      string `json:"name"`
	Points    int    `json:"points"`
	SortOrder int    `json:"sort_order"`
}

func (req *behaviourRequest) validate() string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "name is required"
	}
	if req.Points == 0 {
		return "points must be non-zero"
	}
	return ""
}

func (h *BehaviourHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListBehaviours(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list behaviours")
		return
	}
	if list == nil {
		list = []model.Behaviour{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *BehaviourHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req behaviourRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	b := &model.Behaviour{Name: req.Name, Points: req.Points, SortOrder: req.SortOrder}
	if err := h.store.CreateBehaviour(r.Context(), b); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create behaviour")
		return
	}
	created, err := h.store.GetBehaviour(r.Context(), b.ID)
	if err != nil || created == nil {
		writeJSON(w, http.StatusCreated, b)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *BehaviourHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := h.store.GetBehaviour(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get behaviour")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "behaviour not found")
		return
	}
	var req behaviourRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	existing.Name = req.Name
	existing.Points = req.Points
	existing.SortOrder = req.SortOrder
	if err := h.store.UpdateBehaviour(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update behaviour")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *BehaviourHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteBehaviour(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete behaviour")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Apply credits or debits the behaviour's points to a person. Every
// application is a point_transactions row, so it shows in the activity log
// and can be undone there.
func (h *BehaviourHandler) Apply(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		UserID int64  `json:"user_id"`
		Note   string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	b, err := h.store.GetBehaviour(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get behaviour")
		return
	}
	if b == nil {
		writeError(w, http.StatusNotFound, "behaviour not found")
		return
	}
	user, err := h.store.GetUser(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	tx, err := h.store.ApplyBehaviour(r.Context(), req.UserID, b, strings.TrimSpace(req.Note))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply behaviour")
		return
	}
	writeJSON(w, http.StatusCreated, tx)
}
