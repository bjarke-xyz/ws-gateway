package repository

import (
	"context"
	"time"

	"github.com/bjarke-xyz/ws-gateway/internal/domain"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/samber/lo"
)

type postgresKeyRepository struct {
	conn Connection
}

func NewPostgresKey(conn Connection) domain.ApiKeyRepository {
	return &postgresKeyRepository{
		conn: conn,
	}
}

func mapDtoKey(dto apiKeyDto) domain.ApiKey {
	return domain.ApiKey{
		ID:          dto.ID,
		OwnerUserId: dto.OwnerUserId,
		KeyHash:     dto.KeyHash,
		KeyPreview:  dto.KeyPreview,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}
}
func mapDtoKeys(dtos []apiKeyDto) []domain.ApiKey {
	apiKeyMap := make(map[string]domain.ApiKey)
	for _, dto := range dtos {
		key, ok := apiKeyMap[dto.ID]
		if !ok {
			key = mapDtoKey(dto)
		}
		access := domain.ApiKeyAccess{
			ApiKeyID: dto.ApiKeyId,
			AppID:    dto.AppId,
		}
		if key.Access == nil {
			key.Access = make([]domain.ApiKeyAccess, 0)
		}
		key.Access = append(key.Access, access)
		apiKeyMap[dto.ID] = key
	}
	apiKeys := lo.Values(apiKeyMap)
	return apiKeys
}

// GetByUserID implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) GetByUserID(ctx context.Context, userID string) ([]domain.ApiKey, error) {
	dtoKeys := make([]apiKeyDto, 0)
	query := `
		SELECT k.*, a.* FROM api_keys k
		LEFT JOIN api_key_access a ON a.api_key_id = k.id
		WHERE k.owner_user_id = $1`
	err := pgxscan.Select(ctx, p.conn, &dtoKeys, query, userID)
	keys := mapDtoKeys(dtoKeys)
	return keys, err
}

// GetByAppID implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) GetByAppID(ctx context.Context, appID string) ([]domain.ApiKey, error) {
	dtoKeys := make([]apiKeyDto, 0)
	query := `
		SELECT k.*, a.* FROM api_keys k
		LEFT JOIN api_key_access a ON a.api_key_id = k.id
		WHERE a.app_id = $1`
	err := pgxscan.Select(ctx, p.conn, &dtoKeys, query, appID)
	keys := mapDtoKeys(dtoKeys)
	return keys, err
}

type apiKeyDto struct {
	ID          string
	OwnerUserId string
	KeyHash     string
	KeyPreview  string
	CreatedAt   time.Time
	UpdatedAt   *time.Time

	// From api_key_access table
	ApiKeyId string
	AppId    string
}
