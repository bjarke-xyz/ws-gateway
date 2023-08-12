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
		OwnerUserID: dto.OwnerUserId,
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
		if dto.ApiKeyId != nil && dto.AppId != nil {
			access := domain.ApiKeyAccess{
				ApiKeyID: *dto.ApiKeyId,
				AppID:    *dto.AppId,
			}
			if key.Access == nil {
				key.Access = make([]domain.ApiKeyAccess, 0)
			}
			key.Access = append(key.Access, access)
		}
		apiKeyMap[dto.ID] = key
	}
	apiKeys := lo.Values(apiKeyMap)
	return apiKeys
}

// GetByID implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) GetByID(ctx context.Context, id string) (domain.ApiKey, error) {
	dtoKeys := make([]apiKeyDto, 0)
	query := `
		SELECT k.*, a.* FROM api_keys k
		LEFT JOIN api_key_access a ON a.api_key_id = k.id
		WHERE k.id = $1`
	err := pgxscan.Select(ctx, p.conn, &dtoKeys, query, id)
	if pgxscan.NotFound(err) {
		return domain.ApiKey{}, domain.ErrNotFound
	}
	keys := mapDtoKeys(dtoKeys)
	return keys[0], err
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
	ApiKeyId *string
	AppId    *string
}

// Create implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) Create(ctx context.Context, key *domain.ApiKey) error {
	tx, err := p.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO api_keys (id, owner_user_id, key_hash, key_preview, created_at)
		VALUES ($1, $2, $3, $4, NOW())`, key.ID, key.OwnerUserID, key.KeyHash, key.KeyPreview)
	if err != nil {
		return err
	}

	for _, access := range key.Access {
		_, err = tx.Exec(ctx, `INSERT INTO api_key_access(api_key_id, app_id) VALUES ($1, $2)`, key.ID, access.AppID)
		if err != nil {
			return err
		}
	}
	err = tx.Commit(ctx)
	return err
}

// Update implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) Update(ctx context.Context, apiKeyID string, accessList []domain.ApiKeyAccess) error {
	tx, err := p.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "DELETE FROM api_key_access WHERE api_key_id = $1", apiKeyID)

	for _, access := range accessList {
		_, err = tx.Exec(ctx, `INSERT INTO api_key_access(api_key_id, app_id) VALUES ($1, $2)`, apiKeyID, access.AppID)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	return err
}

// UpdateKeyPreview implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) UpdateKeyPreview(ctx context.Context, apikeyID string, keyPreview string) error {
	_, err := p.conn.Exec(ctx, "UPDATE api_keys SET key_preview = $1 WHERE id = $2", keyPreview, apikeyID)
	return err
}

// Delete implements domain.ApiKeyRepository.
func (p *postgresKeyRepository) Delete(ctx context.Context, keyID string) error {
	tx, err := p.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "DELETE FROM api_key_access WHERE api_key_id = $1", keyID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM api_keys WHERE id = $1", keyID)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return err
}
