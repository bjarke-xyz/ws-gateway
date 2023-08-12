package repository

import (
	"context"

	"github.com/bjarke-xyz/ws-gateway/internal/domain"
	"github.com/georgysavva/scany/v2/pgxscan"
)

type postgresAppRepository struct {
	conn Connection
}

func NewPostgresApp(conn Connection) domain.ApplicationRepository {
	return &postgresAppRepository{conn: conn}
}

// GetByID implements domain.ApplicationRepository.
func (p *postgresAppRepository) GetByID(ctx context.Context, id string) (domain.Application, error) {
	var app domain.Application
	rows, err := p.conn.Query(ctx, "SELECT * FROM apps WHERE id = $1", id)
	if err != nil {
		return app, err
	}
	err = pgxscan.ScanOne(&app, rows)
	if err != nil {
		if pgxscan.NotFound(err) {
			return app, domain.ErrNotFound
		}
		return app, err
	}
	return app, nil
}

// GetByUserID implements domain.ApplicationRepository.
func (p *postgresAppRepository) GetByUserID(ctx context.Context, userId string) ([]domain.Application, error) {
	apps := make([]domain.Application, 0)
	err := pgxscan.Select(ctx, p.conn, &apps, "SELECT * FROM apps WHERE owner_user_id = $1", userId)
	if err != nil {
		return apps, err
	}
	return apps, nil
}

// Update implements domain.ApplicationRepository.
func (p *postgresAppRepository) Update(ctx context.Context, app *domain.Application) error {
	_, err := p.conn.Exec(ctx, "UPDATE apps SET name = $1, updated_at = NOW() WHERE id = $2", app.Name, app.ID)
	return err
}

// Create implements domain.ApplicationRepository.
func (p *postgresAppRepository) Create(ctx context.Context, app *domain.Application) error {
	query := `
		INSERT INTO apps (id, owner_user_id, name, created_at)
		VALUES ($1, $2, $3, NOW())`
	_, err := p.conn.Exec(ctx, query, app.ID, app.OwnerUserID, app.Name)
	return err
}

// Delete implements domain.ApplicationRepository.
func (p *postgresAppRepository) Delete(ctx context.Context, appID string) error {
	_, err := p.conn.Exec(ctx, "DELETE FROM apps WHERE id = $1", appID)
	return err
}
