package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}

func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{
		db: tx,
	}
}

func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		typeNames := []string{
			"user_role",
			"auth_provider",
			"workspace_role",
			"build_pack_type",
			"deployment_status",
			"trigger_source",
			"subscription_status",
			"payment_status",
		}
		for _, typeName := range typeNames {
			dataType, err := conn.LoadType(ctx, typeName)
			if err == nil {
				conn.TypeMap().RegisterType(dataType)
			}
		}
		return nil
	}
	return pgxpool.NewWithConfig(ctx, config)
}
