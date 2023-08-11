package domain

import (
	"context"
	"time"
)

type Application struct {
	ID          string
	OwnerUserID string
	Name        string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

type ApplicationRepository interface {
	GetByID(context.Context, string) (Application, error)
	GetByUserID(context.Context, string) ([]Application, error)
	Update(context.Context, *Application) error
}
