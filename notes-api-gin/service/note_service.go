package service

import (
	"context"
	"errors"
	"strings"

	"notes-api-gin/models"
	"notes-api-gin/repository"
)

var (
	ErrForbidden = errors.New("forbidden: insufficient permissions")
)

type NoteService interface {
	CreateNote(ctx context.Context, userID int64, req models.CreateNoteRequest) (*models.NoteResponse, error)
	GetNote(ctx context.Context, noteID, userID int64) (*models.NoteResponse, error)
	ListNotes(ctx context.Context, userID int64, filter string) ([]models.NoteResponse, error)
	UpdateNote(ctx context.Context, noteID, userID int64, req models.UpdateNoteRequest) (*models.NoteResponse, error)
	DeleteNote(ctx context.Context, noteID, userID int64) error
	ShareNote(ctx context.Context, noteID, ownerID int64, req models.ShareNoteRequest) error
	UnshareNote(ctx context.Context, noteID, ownerID, targetUserID int64) error
	GetNoteShares(ctx context.Context, noteID, ownerID int64) ([]models.SharedUserInfo, error)
	CheckAccess(ctx context.Context, noteID, currentUserID int64, targetUsername string, targetUserID int64) (*models.AccessCheckResponse, error)
}

type noteService struct {
	noteRepo repository.NoteRepository
	userRepo repository.UserRepository
}

func NewNoteService(noteRepo repository.NoteRepository, userRepo repository.UserRepository) NoteService {
	return &noteService{
		noteRepo: noteRepo,
		userRepo: userRepo,
	}
}

func (s *noteService) CreateNote(ctx context.Context, userID int64, req models.CreateNoteRequest) (*models.NoteResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, errors.New("title cannot be empty")
	}

	note, err := s.noteRepo.Create(ctx, userID, req.Title, req.Content)
	if err != nil {
		return nil, err
	}

	user, _ := s.userRepo.GetByID(ctx, userID)
	username := ""
	if user != nil {
		username = user.Username
	}

	return &models.NoteResponse{
		ID:            note.ID,
		Title:         note.Title,
		Content:       note.Content,
		OwnerID:       note.UserID,
		OwnerUsername: username,
		IsOwner:       true,
		IsShared:      false,
		Permission:    "owner",
		CreatedAt:     note.CreatedAt,
		UpdatedAt:     note.UpdatedAt,
	}, nil
}

func (s *noteService) GetNote(ctx context.Context, noteID, userID int64) (*models.NoteResponse, error) {
	isOwner, isShared, perm, ownerID, ownerUsername, err := s.noteRepo.CheckUserAccess(ctx, noteID, userID)
	if err != nil {
		return nil, err
	}

	if !isOwner && !isShared {
		return nil, ErrForbidden
	}

	note, err := s.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return nil, err
	}

	resp := &models.NoteResponse{
		ID:            note.ID,
		Title:         note.Title,
		Content:       note.Content,
		OwnerID:       ownerID,
		OwnerUsername: ownerUsername,
		IsOwner:       isOwner,
		IsShared:      isShared,
		Permission:    perm,
		CreatedAt:     note.CreatedAt,
		UpdatedAt:     note.UpdatedAt,
	}

	if isOwner {
		shares, err := s.noteRepo.GetShares(ctx, noteID)
		if err == nil {
			resp.SharedWith = shares
		}
	}

	return resp, nil
}

func (s *noteService) ListNotes(ctx context.Context, userID int64, filter string) ([]models.NoteResponse, error) {
	return s.noteRepo.ListForUser(ctx, userID, filter)
}

func (s *noteService) UpdateNote(ctx context.Context, noteID, userID int64, req models.UpdateNoteRequest) (*models.NoteResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, errors.New("title cannot be empty")
	}

	isOwner, isShared, perm, ownerID, ownerUsername, err := s.noteRepo.CheckUserAccess(ctx, noteID, userID)
	if err != nil {
		return nil, err
	}

	if !isOwner && (!isShared || perm != "write") {
		return nil, ErrForbidden
	}

	note, err := s.noteRepo.Update(ctx, noteID, req.Title, req.Content)
	if err != nil {
		return nil, err
	}

	return &models.NoteResponse{
		ID:            note.ID,
		Title:         note.Title,
		Content:       note.Content,
		OwnerID:       ownerID,
		OwnerUsername: ownerUsername,
		IsOwner:       isOwner,
		IsShared:      isShared,
		Permission:    perm,
		CreatedAt:     note.CreatedAt,
		UpdatedAt:     note.UpdatedAt,
	}, nil
}

func (s *noteService) DeleteNote(ctx context.Context, noteID, userID int64) error {
	isOwner, _, _, _, _, err := s.noteRepo.CheckUserAccess(ctx, noteID, userID)
	if err != nil {
		return err
	}

	if !isOwner {
		return ErrForbidden
	}

	return s.noteRepo.Delete(ctx, noteID)
}

func (s *noteService) ShareNote(ctx context.Context, noteID, ownerID int64, req models.ShareNoteRequest) error {
	isOwner, _, _, _, _, err := s.noteRepo.CheckUserAccess(ctx, noteID, ownerID)
	if err != nil {
		return err
	}

	if !isOwner {
		return ErrForbidden
	}

	var targetUser *models.User
	if req.UserID > 0 {
		targetUser, err = s.userRepo.GetByID(ctx, req.UserID)
	} else if strings.TrimSpace(req.Username) != "" {
		targetUser, err = s.userRepo.GetByUsername(ctx, strings.TrimSpace(req.Username))
	} else {
		return errors.New("must provide username or user_id to share with")
	}

	if err != nil {
		return errors.New("target user not found")
	}

	if targetUser.ID == ownerID {
		return repository.ErrCannotShareWithSelf
	}

	perm := "read"
	if strings.ToLower(req.Permission) == "write" {
		perm = "write"
	}

	return s.noteRepo.Share(ctx, noteID, targetUser.ID, perm)
}

func (s *noteService) UnshareNote(ctx context.Context, noteID, ownerID, targetUserID int64) error {
	isOwner, _, _, _, _, err := s.noteRepo.CheckUserAccess(ctx, noteID, ownerID)
	if err != nil {
		return err
	}

	if !isOwner {
		return ErrForbidden
	}

	return s.noteRepo.Unshare(ctx, noteID, targetUserID)
}

func (s *noteService) GetNoteShares(ctx context.Context, noteID, ownerID int64) ([]models.SharedUserInfo, error) {
	isOwner, _, _, _, _, err := s.noteRepo.CheckUserAccess(ctx, noteID, ownerID)
	if err != nil {
		return nil, err
	}

	if !isOwner {
		return nil, ErrForbidden
	}

	return s.noteRepo.GetShares(ctx, noteID)
}

func (s *noteService) CheckAccess(ctx context.Context, noteID, currentUserID int64, targetUsername string, targetUserID int64) (*models.AccessCheckResponse, error) {
	isOwner, isShared, perm, ownerID, ownerUsername, err := s.noteRepo.CheckUserAccess(ctx, noteID, currentUserID)
	if err != nil {
		return nil, err
	}

	checkUserID := currentUserID
	if targetUserID > 0 || strings.TrimSpace(targetUsername) != "" {
		if !isOwner {
			return nil, ErrForbidden
		}
		var targetUser *models.User
		if targetUserID > 0 {
			targetUser, err = s.userRepo.GetByID(ctx, targetUserID)
		} else {
			targetUser, err = s.userRepo.GetByUsername(ctx, strings.TrimSpace(targetUsername))
		}
		if err != nil {
			return nil, errors.New("target user not found")
		}
		checkUserID = targetUser.ID

		isOwner, isShared, perm, _, _, err = s.noteRepo.CheckUserAccess(ctx, noteID, checkUserID)
		if err != nil {
			return nil, err
		}
	}

	checkedUser, _ := s.userRepo.GetByID(ctx, checkUserID)
	cUsername := ""
	if checkedUser != nil {
		cUsername = checkedUser.Username
	}

	canRead := isOwner || isShared
	canWrite := isOwner || (isShared && perm == "write")
	canDelete := isOwner
	canShare := isOwner

	return &models.AccessCheckResponse{
		NoteID:        noteID,
		UserID:        checkUserID,
		Username:      cUsername,
		IsOwner:       isOwner,
		IsShared:      isShared,
		Permission:    perm,
		CanRead:       canRead,
		CanWrite:      canWrite,
		CanDelete:     canDelete,
		CanShare:      canShare,
		OwnerID:       ownerID,
		OwnerUsername: ownerUsername,
	}, nil
}

