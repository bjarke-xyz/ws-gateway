package domain

import (
	"context"
	"time"
)

type ApiKey struct {
	ID          string
	OwnerUserId string
	KeyHash     string
	KeyPreview  string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	Access      []ApiKeyAccess
}

type ApiKeyAccess struct {
	ApiKeyID string
	AppID    string
}

type ApiKeyRepository interface {
	GetByUserID(context.Context, string) ([]ApiKey, error)
	GetByAppID(context.Context, string) ([]ApiKey, error)
}
