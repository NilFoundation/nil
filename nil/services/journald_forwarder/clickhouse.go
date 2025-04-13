package journald_forwarder

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common/logging"
)

const (
	DefaultDatabase = "nil"
	DefaultTable    = "events"
)

type ClickhouseConfig struct {
	ClickhouseAddr string
	ListenAddr     string
	DbUser         string
	DbDatabase     string
	DbPassword     string
}

type Clickhouse struct {
	ClickhouseConfig
	connection clickhouse.Conn
}

func (s *Clickhouse) InsertColumnsInTable(
	ctx context.Context,
	database, tableName string,
	columns, columnsType []string,
) error {
	columnClauses := make([]string, len(columns))
	for i, column := range columns {
		columnClauses[i] = fmt.Sprintf("ADD COLUMN %s %s", column, columnsType[i])
	}

	query := fmt.Sprintf(
		"ALTER TABLE %s.%s %s;",
		database,
		tableName,
		strings.Join(columnClauses, ", "),
	)

	return s.connection.Exec(ctx, query)
}

func (s *Clickhouse) InsertData(
	ctx context.Context,
	database string,
	tableName string,
	columns []string,
	values [][]any,
) error {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}

	// Create multiple value placeholders for batch insert
	valueRows := make([]string, len(values))
	for i := range values {
		valueRows[i] = fmt.Sprintf("(%s)", strings.Join(placeholders, ", "))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES %s",
		database,
		tableName,
		strings.Join(columns, ", "),
		strings.Join(valueRows, ", "),
	)

	// Flatten values slice
	flattenedValues := make([]any, 0, len(values)*len(columns))
	for _, row := range values {
		flattenedValues = append(flattenedValues, row...)
	}

	return s.connection.Exec(ctx, query, flattenedValues...)
}

func (s *Clickhouse) GetTabelColumnNames(ctx context.Context, database, tableName string) (map[string]any, error) {
	const columnQuery = "SELECT name FROM system.columns WHERE database = ? AND table = ?"

	rows, err := s.connection.Query(ctx, columnQuery, database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]any)
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		columns[columnName] = nil
	}
	return columns, nil
}

func (s *Clickhouse) InitializeDatabaseSchema(ctx context.Context, logger logging.Logger) error {
	operations := []struct {
		name   string
		action func(context.Context, driver.Conn) error
		errMsg string
	}{
		{
			name: "create database",
			action: func(ctx context.Context, conn driver.Conn) error {
				return conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", DefaultDatabase))
			},
			errMsg: "Failed to create database",
		},
		{
			name: "create table",
			action: func(ctx context.Context, conn driver.Conn) error {
				query := fmt.Sprintf(
					"CREATE TABLE IF NOT EXISTS %s.%s (time DateTime64 DEFAULT now()) "+
						"ENGINE = MergeTree() ORDER BY time",
					DefaultDatabase, DefaultTable)
				return conn.Exec(ctx, query)
			},
			errMsg: "Failed to create table",
		},
	}

	for _, op := range operations {
		if err := op.action(ctx, s.connection); err != nil {
			logger.Error().Err(err).Msg(op.errMsg)
			return fmt.Errorf("%s: %w", op.name, err)
		}
	}
	return nil
}

func (s *Clickhouse) Connect() error {
	var err error
	s.connection, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: s.DbDatabase,
			Username: s.DbUser,
			Password: s.DbPassword,
		},
	})
	return err
}

func (s *Clickhouse) Close() error {
	return s.connection.Close()
}

func NewClickhouse(config ClickhouseConfig) *Clickhouse {
	return &Clickhouse{
		ClickhouseConfig: config,
	}
}
