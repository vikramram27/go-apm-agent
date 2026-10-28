package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"notes-api/models"
)

var (
	ErrNoteNotFound       = errors.New("note not found")
	ErrUnauthorized       = errors.New("unauthorized to access or modify this note")
	ErrCannotShareWithSelf = errors.New("cannot share note with yourself")
)

type NoteRepository interface {
	Create(ctx context.Context, userID int64, title, content string) (*models.Note, error)
	GetByID(ctx context.Context, id int64) (*models.Note, error)
	ListForUser(ctx context.Context, userID int64, filter string) ([]models.NoteResponse, error)
	Update(ctx context.Context, id int64, title, content string) (*models.Note, error)
	Delete(ctx context.Context, id int64) error
	Share(ctx context.Context, noteID, sharedWithUserID int64, permission string) error
	Unshare(ctx context.Context, noteID, sharedWithUserID int64) error
	GetShares(ctx context.Context, noteID int64) ([]models.SharedUserInfo, error)
	CheckUserAccess(ctx context.Context, noteID, userID int64) (isOwner bool, isShared bool, permission string, ownerID int64, ownerUsername string, err error)
}

type PostgresNoteRepository struct {
	db *sql.DB
}

func NewNoteRepository(db *sql.DB) NoteRepository {
	return &PostgresNoteRepository{db: db}
}

func (r *PostgresNoteRepository) Create(ctx context.Context, userID int64, title, content string) (*models.Note, error) {
	query := `
		INSERT INTO notes (user_id, title, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, title, content, created_at, updated_at
	`
	now := time.Now()
	note := &models.Note{}
	err := r.db.QueryRowContext(ctx, query, userID, title, content, now, now).Scan(
		&note.ID,
		&note.UserID,
		&note.Title,
		&note.Content,
		&note.CreatedAt,
		&note.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (r *PostgresNoteRepository) GetByID(ctx context.Context, id int64) (*models.Note, error) {
	query := `
		SELECT id, user_id, title, content, created_at, updated_at
		FROM notes
		WHERE id = $1
	`
	note := &models.Note{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&note.ID,
		&note.UserID,
		&note.Title,
		&note.Content,
		&note.CreatedAt,
		&note.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (r *PostgresNoteRepository) ListForUser(ctx context.Context, userID int64, filter string) ([]models.NoteResponse, error) {
	var query string
	var args []interface{}

	switch filter {
	case "owned":
		query = `
			SELECT n.id, n.title, n.content, n.user_id, u.username,
			       TRUE AS is_owner, FALSE AS is_shared, 'owner' AS permission,
			       n.created_at, n.updated_at
			FROM notes n
			JOIN users u ON n.user_id = u.id
			WHERE n.user_id = $1
			ORDER BY n.updated_at DESC
		`
		args = []interface{}{userID}

	case "shared":
		query = `
			SELECT n.id, n.title, n.content, n.user_id, u.username,
			       FALSE AS is_owner, TRUE AS is_shared, ns.permission,
			       n.created_at, n.updated_at
			FROM notes n
			JOIN users u ON n.user_id = u.id
			JOIN note_shares ns ON n.id = ns.note_id
			WHERE ns.shared_with_user_id = $1
			ORDER BY n.updated_at DESC
		`
		args = []interface{}{userID}

	default: // "all"
		query = `
			SELECT n.id, n.title, n.content, n.user_id, u.username,
			       TRUE AS is_owner, FALSE AS is_shared, 'owner' AS permission,
			       n.created_at, n.updated_at
			FROM notes n
			JOIN users u ON n.user_id = u.id
			WHERE n.user_id = $1

			UNION ALL

			SELECT n.id, n.title, n.content, n.user_id, u.username,
			       FALSE AS is_owner, TRUE AS is_shared, ns.permission,
			       n.created_at, n.updated_at
			FROM notes n
			JOIN users u ON n.user_id = u.id
			JOIN note_shares ns ON n.id = ns.note_id
			WHERE ns.shared_with_user_id = $1
			ORDER BY updated_at DESC
		`
		args = []interface{}{userID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]models.NoteResponse, 0)
	for rows.Next() {
		var n models.NoteResponse
		if err := rows.Scan(
			&n.ID,
			&n.Title,
			&n.Content,
			&n.OwnerID,
			&n.OwnerUsername,
			&n.IsOwner,
			&n.IsShared,
			&n.Permission,
			&n.CreatedAt,
			&n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func (r *PostgresNoteRepository) Update(ctx context.Context, id int64, title, content string) (*models.Note, error) {
	query := `
		UPDATE notes
		SET title = $1, content = $2, updated_at = $3
		WHERE id = $4
		RETURNING id, user_id, title, content, created_at, updated_at
	`
	note := &models.Note{}
	err := r.db.QueryRowContext(ctx, query, title, content, time.Now(), id).Scan(
		&note.ID,
		&note.UserID,
		&note.Title,
		&note.Content,
		&note.CreatedAt,
		&note.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (r *PostgresNoteRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM notes WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func (r *PostgresNoteRepository) Share(ctx context.Context, noteID, sharedWithUserID int64, permission string) error {
	if permission != "write" {
		permission = "read"
	}
	query := `
		INSERT INTO note_shares (note_id, shared_with_user_id, permission, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (note_id, shared_with_user_id)
		DO UPDATE SET permission = EXCLUDED.permission
	`
	_, err := r.db.ExecContext(ctx, query, noteID, sharedWithUserID, permission, time.Now())
	return err
}

func (r *PostgresNoteRepository) Unshare(ctx context.Context, noteID, sharedWithUserID int64) error {
	query := `DELETE FROM note_shares WHERE note_id = $1 AND shared_with_user_id = $2`
	res, err := r.db.ExecContext(ctx, query, noteID, sharedWithUserID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("share relation not found")
	}
	return nil
}

func (r *PostgresNoteRepository) GetShares(ctx context.Context, noteID int64) ([]models.SharedUserInfo, error) {
	query := `
		SELECT u.id, u.username, u.email, ns.permission, ns.created_at
		FROM note_shares ns
		JOIN users u ON ns.shared_with_user_id = u.id
		WHERE ns.note_id = $1
		ORDER BY ns.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shares := make([]models.SharedUserInfo, 0)
	for rows.Next() {
		var s models.SharedUserInfo
		if err := rows.Scan(&s.UserID, &s.Username, &s.Email, &s.Permission, &s.SharedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	return shares, rows.Err()
}

func (r *PostgresNoteRepository) CheckUserAccess(ctx context.Context, noteID, userID int64) (isOwner bool, isShared bool, permission string, ownerID int64, ownerUsername string, err error) {
	// First check note existence and ownership
	query := `
		SELECT n.user_id, u.username
		FROM notes n
		JOIN users u ON n.user_id = u.id
		WHERE n.id = $1
	`
	err = r.db.QueryRowContext(ctx, query, noteID).Scan(&ownerID, &ownerUsername)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "none", 0, "", ErrNoteNotFound
	}
	if err != nil {
		return false, false, "none", 0, "", err
	}

	if ownerID == userID {
		return true, false, "owner", ownerID, ownerUsername, nil
	}

	// Check if shared
	shareQuery := `
		SELECT permission
		FROM note_shares
		WHERE note_id = $1 AND shared_with_user_id = $2
	`
	var perm string
	err = r.db.QueryRowContext(ctx, shareQuery, noteID, userID).Scan(&perm)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "none", ownerID, ownerUsername, nil
	}
	if err != nil {
		return false, false, "none", ownerID, ownerUsername, err
	}

	return false, true, perm, ownerID, ownerUsername, nil
}

