package journald_forwarder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
	logs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	v1 "go.opentelemetry.io/proto/otlp/common/v1"
	v12 "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
)

const (
	DefaultDatabase = "nil"
	DefaultTable    = "events"
)

type Config struct {
	ClickhouseAddr string
	ListenAddr     string
	DbUser         string
	DbDatabase     string
	DbPassword     string
}

type LogServer struct {
	logs.UnimplementedLogsServiceServer
	connect driver.Conn
	logger  logging.Logger
}

func NewLogServer(connect driver.Conn, logger logging.Logger) *LogServer {
	return &LogServer{connect: connect, logger: logger}
}

func insertColumnsInTable(ctx context.Context,
	connect driver.Conn, database, tableName string,
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

	return connect.Exec(ctx, query)
}

func insertData(
	ctx context.Context,
	connect driver.Conn,
	database string,
	tableName string,
	columns []string,
	values []any,
) error {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s)",
		database,
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	return connect.Exec(ctx, query, values...)
}

func getTabelColumnNames(ctx context.Context, connect driver.Conn, database, tableName string) ([]string, error) {
	const columnQuery = "SELECT name FROM system.columns WHERE database = ? AND table = ?"

	rows, err := connect.Query(ctx, columnQuery, database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		columns = append(columns, columnName)
	}
	return columns, nil
}

var fieldStoreClickhouseTyped = logging.FieldStoreToClickhouse + logging.GetAbbreviation("bool")

func extractLogColumns(data map[string]any) ([]string, []string, []any) {
	columns := make([]string, 0, len(data))
	columnTypes := make([]string, 0, len(data))
	values := make([]any, 0, len(data))

	for key, value := range data {
		baseColumn := key[:len(key)-logging.LogAbbreviationSize]
		columnType := key[len(key)-logging.LogAbbreviationSize:]

		columns = append(columns, baseColumn)
		columnTypes = append(columnTypes, columnType)
		values = append(values, value)
	}

	return columns, columnTypes, values
}

func storeData(ctx context.Context, logger logging.Logger, connect driver.Conn, data map[string]any) error {
	delete(data, fieldStoreClickhouseTyped)
	for _, field := range []string{
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
		zerolog.LevelFieldName,
		zerolog.ErrorFieldName,
		zerolog.ErrorStackFieldName,
		logging.FieldHostName,
		logging.FieldSystemdUnit,
	} {
		if value, ok := data[field]; ok {
			data[field+logging.GetAbbreviation("string")] = value
			delete(data, field)
		}
	}

	if value, ok := data[zerolog.TimestampFieldName]; ok {
		strValue, ok := value.(string)
		if !ok {
			logger.Error().Msgf("timestamp is not a string in log %+v", data)
			return errors.New("timestamp is not a string")
		}
		parsedTime, err := time.Parse(time.RFC3339, strValue)
		if err != nil {
			logger.Error().Err(err).Msg("Error parsing timestamp")
			return err
		}
		data[zerolog.TimestampFieldName+logging.GetAbbreviation("datetime64")] = parsedTime
		delete(data, zerolog.TimestampFieldName)
	}

	columns, columnsDef, values := extractLogColumns(data)

	for {
		if err := insertData(ctx, connect, DefaultDatabase, DefaultTable, columns, values); err != nil {
			columNames, err := getTabelColumnNames(ctx, connect, DefaultDatabase, DefaultTable)
			if err != nil {
				logger.Error().Err(err).Msg("Error getting table columns")
				return err
			}
			var diff []string
			var typeDiff []string
			for i, col1 := range columns {
				idx := slices.Index(columNames, col1)
				if idx == -1 {
					diff = append(diff, col1)
					chType, err := logging.GetClickhouseByAbbreviation(columnsDef[i])
					if err != nil {
						logger.Error().Err(err).
							Msgf("clickhouse type error: log %+v, column: %s", data, col1)
						return err
					}
					typeDiff = append(typeDiff, chType)
				}
			}
			if len(diff) == 0 {
				logger.Error().Err(err).Msg("Error inserting data")
				return err
			}
			if err := insertColumnsInTable(ctx, connect, DefaultDatabase, DefaultTable, diff, typeDiff); err != nil {
				logger.Error().Err(err).Msg("Error inserting columns")
				return err
			}
		} else {
			break
		}
	}
	return nil
}

func (s *LogServer) processResourceLog(ctx context.Context, resourceLog *v12.ResourceLogs) error {
	for _, scopeLog := range resourceLog.ScopeLogs {
		if err := s.processScopeLog(ctx, scopeLog); err != nil {
			return err
		}
	}
	return nil
}

func (s *LogServer) processScopeLog(ctx context.Context, scopeLog *v12.ScopeLogs) error {
	for _, logRecord := range scopeLog.LogRecords {
		if err := s.processLogRecord(ctx, logRecord); err != nil {
			return err
		}
	}
	return nil
}

func (s *LogServer) processLogRecord(ctx context.Context, logRecord *v12.LogRecord) error {
	var jsonData *v1.AnyValue
	var hostname string
	var unit string
	for _, kv := range logRecord.Body.GetKvlistValue().GetValues() {
		switch kv.GetKey() {
		case "JSON":
			jsonData = kv.GetValue()
		case logging.FieldHostName:
			hostname = kv.GetValue().GetStringValue()
		case logging.FieldSystemdUnit:
			unit = kv.GetValue().GetStringValue()
		}
		if kv.GetKey() == "JSON" {
			jsonData = kv.GetValue()
			break
		}
	}
	if jsonData == nil || jsonData.GetStringValue() == "" {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonData.GetStringValue()), &data); err != nil {
		s.logger.Error().Err(err).Msg("Error parsing JSON")
		return nil
	}

	if data[fieldStoreClickhouseTyped] == true {
		data[logging.FieldHostName] = hostname
		data[logging.FieldSystemdUnit] = unit
		return storeData(ctx, s.logger, s.connect, data)
	}
	return nil
}

func (s *LogServer) Export(
	ctx context.Context, req *logs.ExportLogsServiceRequest,
) (*logs.ExportLogsServiceResponse, error) {
	for _, resourceLog := range req.ResourceLogs {
		if err := s.processResourceLog(ctx, resourceLog); err != nil {
			return nil, err
		}
	}
	return &logs.ExportLogsServiceResponse{}, nil
}

func initializeDatabaseSchema(ctx context.Context, connect driver.Conn, logger logging.Logger) error {
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
		if err := op.action(ctx, connect); err != nil {
			logger.Error().Err(err).Msg(op.errMsg)
			return fmt.Errorf("%s: %w", op.name, err)
		}
	}
	return nil
}

func Run(ctx context.Context, cfg Config, logger logging.Logger) error {
	connect, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: cfg.DbDatabase,
			Username: cfg.DbUser,
			Password: cfg.DbPassword,
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to connect to ClickHouse")
		return err
	}
	defer connect.Close()

	if err := initializeDatabaseSchema(ctx, connect, logger); err != nil {
		return err
	}

	server := grpc.NewServer()
	logs.RegisterLogsServiceServer(server, NewLogServer(connect, logger))

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to listen on port %s", cfg.ListenAddr)
		return err
	}
	logger.Info().Str("listen_addr", cfg.ListenAddr).Msg("Log receiver listening")

	serverErrChan := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	select {
	case <-ctx.Done():
		logger.Info().Msg("Shutting down gRPC server due to context cancellation")
		server.GracefulStop()
		return nil
	case err := <-serverErrChan:
		logger.Error().Err(err).Msg("gRPC server failed")
		return err
	}
}
