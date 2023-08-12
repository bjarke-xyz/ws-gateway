package domain

import (
	"context"
	"time"
)

type ApiKey struct {
	ID          string
	OwnerUserID string
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
	GetByID(context.Context, string) (ApiKey, error)
	GetByUserID(context.Context, string) ([]ApiKey, error)
	GetByAppID(context.Context, string) ([]ApiKey, error)
	Create(context.Context, *ApiKey) error
	Update(ctx context.Context, apiKeyID string, accessList []ApiKeyAccess) error
	UpdateKeyPreview(ctx context.Context, apikeyID string, keyPreview string) error
	Delete(context.Context, string) error
}
