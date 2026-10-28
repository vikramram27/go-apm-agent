package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"notes-api/middleware"
	"notes-api/models"
	"notes-api/repository"
	"notes-api/service"
)

type NoteHandler struct {
	noteService service.NoteService
}

func NewNoteHandler(noteService service.NoteService) *NoteHandler {
	return &NoteHandler{noteService: noteService}
}

func (h *NoteHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req models.CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	note, err := h.noteService.CreateNote(r.Context(), user.ID, req)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusCreated, note)
}

func (h *NoteHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	filter := r.URL.Query().Get("filter")
	notes, err := h.noteService.ListNotes(r.Context(), user.ID, filter)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(notes),
		"notes": notes,
	})
}

func (h *NoteHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	note, err := h.noteService.GetNote(r.Context(), noteID, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: note not shared with you"})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, note)
}

func (h *NoteHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	var req models.UpdateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	note, err := h.noteService.UpdateNote(r.Context(), noteID, user.ID, req)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: write permission required"})
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, note)
}

func (h *NoteHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	err = h.noteService.DeleteNote(r.Context(), noteID, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: only owner can delete note"})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"message": "note deleted successfully"})
}

func (h *NoteHandler) ShareNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	var req models.ShareNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	err = h.noteService.ShareNote(r.Context(), noteID, user.ID, req)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: only owner can share note"})
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "note shared successfully",
		"note_id":    noteID,
		"permission": req.Permission,
	})
}

func (h *NoteHandler) GetNoteShares(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	shares, err := h.noteService.GetNoteShares(r.Context(), noteID, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: only owner can view shares"})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"note_id": noteID,
		"count":   len(shares),
		"shares":  shares,
	})
}

func (h *NoteHandler) UnshareNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	targetUserID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid target user ID"})
		return
	}

	err = h.noteService.UnshareNote(r.Context(), noteID, user.ID, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied: only owner can unshare note"})
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"message": "note share revoked successfully"})
}

func (h *NoteHandler) CheckAccess(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetAuthUser(r)
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}

	targetUsername := r.URL.Query().Get("target_username")
	targetUserIDStr := r.URL.Query().Get("target_user_id")
	var targetUserID int64
	if targetUserIDStr != "" {
		targetUserID, _ = strconv.ParseInt(targetUserIDStr, 10, 64)
	}

	access, err := h.noteService.CheckAccess(r.Context(), noteID, user.ID, targetUsername, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sendJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, access)
}

