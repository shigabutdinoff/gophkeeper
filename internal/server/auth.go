package server

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type auth struct {
	db   *gorm.DB
	key  *ecdsa.PrivateKey
	kid  string
	keys []byte
}

func open(ctx context.Context, s Settings) (*auth, error) {
	a, err := loadKey(s.TokenKey)
	if err != nil {
		return nil, fmt.Errorf("ключ подписи токенов не прочитан: %w", err)
	}
	if a.db, err = gorm.Open(postgres.Open(s.Database), &gorm.Config{Logger: logger.Discard, DisableAutomaticPing: true}); err != nil {
		return nil, fmt.Errorf("адрес базы данных не разобран: %w", err)
	}
	db, _ := a.db.DB()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err == nil {
		_, err = p.Up(context.WithoutCancel(ctx))
	}
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("база данных недоступна: %w", err)
	}
	return a, nil
}
