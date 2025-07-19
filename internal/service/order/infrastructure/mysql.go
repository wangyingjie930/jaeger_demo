package infrastructure

import (
	"context"
	"nexus/internal/service/order/domain"
)

type MysqlRepository struct {
}

func NewMysqlRepository() *MysqlRepository {
	return &MysqlRepository{}
}

func (m MysqlRepository) Save(ctx context.Context, order *domain.Order) error {
	return nil
}

func (m MysqlRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	return nil, nil
}

func (m MysqlRepository) UpdateState(ctx context.Context, id string, state domain.State) error {
	return nil
}
