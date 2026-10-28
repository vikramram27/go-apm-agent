package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"notes-api/cache"
	"notes-api/config"
	"notes-api/models"
	"notes-api/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTokenRevoked        = errors.New("token has been revoked")
	ErrInvalidToken        = errors.New("invalid or expired token")
)

type CustomClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

type AuthService interface {
	Register(ctx context.Context, req models.RegisterRequest) (*models.UserResponse, error)
	Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error)
	Logout(ctx context.Context, tokenStr string) error
	ValidateToken(ctx context.Context, tokenStr string) (*models.UserResponse, *CustomClaims, error)
	GetUserByID(ctx context.Context, id int64) (*models.UserResponse, error)
}

type authService struct {
	userRepo  repository.UserRepository
	authCache cache.AuthCache
	cfg       *config.Config
}

func NewAuthService(userRepo repository.UserRepository, authCache cache.AuthCache, cfg *config.Config) AuthService {
	return &authService{
		userRepo:  userRepo,
		authCache: authCache,
		cfg:       cfg,
	}
}

func (s *authService) Register(ctx context.Context, req models.RegisterRequest) (*models.UserResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Username == "" || req.Email == "" || len(req.Password) < 6 {
		return nil, errors.New("username and email are required; password must be at least 6 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, req.Username, req.Email, string(hash))
	if err != nil {
		return nil, err
	}

	return &models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *authService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}

	user, err := s.userRepo.GetByUsernameOrEmail(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	jti, err := generateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	expiresDuration := time.Duration(s.cfg.JWTExpirationHours) * time.Hour
	expiresAt := time.Now().Add(expiresDuration)

	claims := CustomClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "notes-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	userResp := models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	// Cache user session in Redis
	if err := s.authCache.SetUserSession(ctx, jti, &userResp, expiresDuration); err != nil {
		// Log warning, don't necessarily abort login
		fmt.Printf("Warning: failed to cache session in Redis: %v\n", err)
	}

	return &models.AuthResponse{
		Token:     tokenString,
		TokenType: "Bearer",
		ExpiresIn: int64(expiresDuration.Seconds()),
		User:      userResp,
	}, nil
}

func (s *authService) Logout(ctx context.Context, tokenStr string) error {
	claims, err := s.parseTokenClaims(tokenStr)
	if err != nil {
		return err
	}

	jti := claims.ID
	if jti == "" {
		return errors.New("token missing jti")
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		ttl = time.Minute
	}

	// Invalidate session in Redis
	if err := s.authCache.BlacklistToken(ctx, jti, ttl); err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}
	_ = s.authCache.DeleteUserSession(ctx, jti)

	return nil
}

func (s *authService) ValidateToken(ctx context.Context, tokenStr string) (*models.UserResponse, *CustomClaims, error) {
	claims, err := s.parseTokenClaims(tokenStr)
	if err != nil {
		return nil, nil, err
	}

	jti := claims.ID
	// Check Redis blacklist
	blacklisted, err := s.authCache.IsTokenBlacklisted(ctx, jti)
	if err == nil && blacklisted {
		return nil, nil, ErrTokenRevoked
	}

	// Check Redis session cache
	cachedUser, err := s.authCache.GetUserSession(ctx, jti)
	if err == nil && cachedUser != nil {
		return cachedUser, claims, nil
	}

	// Fallback to DB if cache missed
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	userResp := &models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	// Refresh cache
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		_ = s.authCache.SetUserSession(ctx, jti, userResp, ttl)
	}

	return userResp, claims, nil
}

func (s *authService) GetUserByID(ctx context.Context, id int64) (*models.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *authService) parseTokenClaims(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func generateRandomHex(byteCount int) (string, error) {
	b := make([]byte, byteCount)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

