package common

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func CreateClickHouseDbIfNotExists(ctx context.Context, dbname, user, password, endpoint string) error {
	connectionOptions := clickhouse.Options{
		Auth: clickhouse.Auth{
			Database: "system",
			Username: user,
			Password: password,
		},
		Addr: []string{endpoint},
	}
	conn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbname))
}
