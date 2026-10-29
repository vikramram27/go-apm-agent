package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"notes-api/config"
	"notes-api/models"
)

type AuthCache interface {
	SetUserSession(ctx context.Context, jti string, user *models.UserResponse, ttl time.Duration) error
	GetUserSession(ctx context.Context, jti string) (*models.UserResponse, error)
	DeleteUserSession(ctx context.Context, jti string) error
	BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)

	// Refresh token storage in Redis
	SetRefreshToken(ctx context.Context, refreshToken string, user *models.UserResponse, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, refreshToken string) (*models.UserResponse, error)
	DeleteRefreshToken(ctx context.Context, refreshToken string) error

	Ping(ctx context.Context) error
}

type RedisAuthCache struct {
	client *redis.Client
}

func ConnectRedis(cfg *config.Config) (*RedisAuthCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	maxAttempts := 10
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = rdb.Ping(ctx).Err()
		if err == nil {
			log.Println("Successfully connected to Redis")
			break
		}
		log.Printf("Redis connection attempt %d/%d failed: %v. Retrying in 2s...", attempt, maxAttempts, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to Redis after %d attempts: %w", maxAttempts, err)
	}

	return &RedisAuthCache{client: rdb}, nil
}

func (r *RedisAuthCache) SetUserSession(ctx context.Context, jti string, user *models.UserResponse, ttl time.Duration) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("auth:session:%s", jti)
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisAuthCache) GetUserSession(ctx context.Context, jti string) (*models.UserResponse, error) {
	key := fmt.Sprintf("auth:session:%s", jti)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found or expired
	}
	if err != nil {
		return nil, err
	}

	var user models.UserResponse
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *RedisAuthCache) DeleteUserSession(ctx context.Context, jti string) error {
	key := fmt.Sprintf("auth:session:%s", jti)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisAuthCache) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("auth:blacklist:%s", jti)
	return r.client.Set(ctx, key, "revoked", ttl).Err()
}

func (r *RedisAuthCache) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("auth:blacklist:%s", jti)
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "revoked", nil
}

func (r *RedisAuthCache) SetRefreshToken(ctx context.Context, refreshToken string, user *models.UserResponse, ttl time.Duration) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("auth:refresh:%s", refreshToken)
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisAuthCache) GetRefreshToken(ctx context.Context, refreshToken string) (*models.UserResponse, error) {
	key := fmt.Sprintf("auth:refresh:%s", refreshToken)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found or expired
	}
	if err != nil {
		return nil, err
	}

	var user models.UserResponse
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *RedisAuthCache) DeleteRefreshToken(ctx context.Context, refreshToken string) error {
	key := fmt.Sprintf("auth:refresh:%s", refreshToken)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisAuthCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
