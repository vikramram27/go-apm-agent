package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"` // Can be username or email
	Password string `json:"password"`
}

type AuthResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"` // seconds
	User         UserResponse `json:"user"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"` // seconds
	User         UserResponse `json:"user"`
}

type Note struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NoteShare struct {
	ID               int64     `json:"id"`
	NoteID           int64     `json:"note_id"`
	SharedWithUserID int64     `json:"shared_with_user_id"`
	Permission       string    `json:"permission"` // "read" or "write"
	CreatedAt        time.Time `json:"created_at"`
}

type SharedUserInfo struct {
	UserID     int64     `json:"user_id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Permission string    `json:"permission"`
	SharedAt   time.Time `json:"shared_at"`
}

type NoteResponse struct {
	ID            int64            `json:"id"`
	Title         string           `json:"title"`
	Content       string           `json:"content"`
	OwnerID       int64            `json:"owner_id"`
	OwnerUsername string           `json:"owner_username"`
	IsOwner       bool             `json:"is_owner"`
	IsShared      bool             `json:"is_shared"`
	Permission    string           `json:"permission"` // "owner", "read", "write"
	SharedWith    []SharedUserInfo `json:"shared_with,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type CreateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type UpdateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ShareNoteRequest struct {
	Username   string `json:"username,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	Permission string `json:"permission"` // "read" (default) or "write"
}

type AccessCheckResponse struct {
	NoteID         int64  `json:"note_id"`
	UserID         int64  `json:"user_id"`
	Username       string `json:"username"`
	IsOwner        bool   `json:"is_owner"`
	IsShared       bool   `json:"is_shared"`
	Permission     string `json:"permission"` // "none", "owner", "read", "write"
	CanRead        bool   `json:"can_read"`
	CanWrite       bool   `json:"can_write"`
	CanDelete      bool   `json:"can_delete"`
	CanShare       bool   `json:"can_share"`
	OwnerID        int64  `json:"owner_id"`
	OwnerUsername  string `json:"owner_username"`
}

