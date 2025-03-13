package journald_forwarder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

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
	logger  zerolog.Logger
}

func NewLogServer(connect driver.Conn, logger zerolog.Logger) *LogServer {
	return &LogServer{connect: connect, logger: logger}
}

func createDatabaseIfNotExists(ctx context.Context, connect driver.Conn, database string) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", database)
	return connect.Exec(ctx, query)
}

func createTableIfNotExists(ctx context.Context, connect driver.Conn, database, tableName string) error {
	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (time DateTime DEFAULT now()) ENGINE = MergeTree() ORDER BY time",
		database,
		tableName,
	)
	return connect.Exec(ctx, query)
}

func insertColumnsInTable(ctx context.Context, connect driver.Conn, database, tableName string, columns, columnsType []string) error {
	query := fmt.Sprintf(
		"ALTER TABLE %s.%s",
		database,
		tableName,
	)
	firstOp := true
	for i, column := range columns {
		if !firstOp {
			query += ","
		} else {
			firstOp = false
		}
		query += fmt.Sprintf(" ADD COLUMN %s %s", column, columnsType[i])
	}
	query += ";"
	return connect.Exec(ctx, query)
}

func insertData(ctx context.Context, connect driver.Conn, database, tableName string, columns []string, values []any) error {
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
	query := fmt.Sprintf(
		"SELECT name FROM system.columns WHERE database = '%s' AND table = '%s'",
		database,
		tableName,
	)
	rows, err := connect.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		res = append(res, columnName)
	}
	return res, nil
}

func storeData(ctx context.Context, logger zerolog.Logger, connect driver.Conn, data map[string]any) {
	delete(data, "store_to_clickhouse"+logging.GetAbbreviation("bool"))
	// caller
	// time
	// message
	for _, field := range []string{"caller", "time", "message"} {
		if value, ok := data[field]; ok {
			data[field+logging.GetAbbreviation("string")] = value
			delete(data, field)
		}
	}

	columns := []string{}
	columnsDef := []string{}
	var values []any
	for key, value := range data {
		columns = append(columns, key[0:len(key)-2])
		columnsDef = append(columnsDef, key[len(key)-2:])
		values = append(values, value)
	}

	if err := insertData(ctx, connect, DefaultDatabase, DefaultTable, columns, values); err != nil {
		columNames, err := getTabelColumnNames(ctx, connect, DefaultDatabase, DefaultTable)
		if err != nil {
			logger.Error().Err(err).Msg("Error getting table columns")
			return
		}
		var diff []string
		var typeDiff []string
		for i, col1 := range columns {
			found := false
			for _, col2 := range columNames {
				if col1 == col2 {
					found = true
					break
				}
			}
			if !found {
				diff = append(diff, col1)
				abbreviation, err := logging.GetClickhouseByAbbreviation(columnsDef[i])
				if err != nil {
					logger.Error().Err(err).Msg("Error getting clickhouse type")
					return
				}
				typeDiff = append(typeDiff, abbreviation)
			}
		}
		if len(diff) == 0 {
			logger.Error().Err(err).Msg("Error inserting data")
			return
		}
		if err := insertColumnsInTable(ctx, connect, DefaultDatabase, DefaultTable, diff, typeDiff); err != nil {
			logger.Error().Err(err).Msg("Error inserting columns")
			return
		}
	}

	log.Printf("Inserted data: %v", data)
}

func (s *LogServer) processResourceLog(ctx context.Context, resourceLog *v12.ResourceLogs) {
	for _, scopeLog := range resourceLog.ScopeLogs {
		s.processScopeLog(ctx, scopeLog)
	}
}

func (s *LogServer) processScopeLog(ctx context.Context, scopeLog *v12.ScopeLogs) {
	for _, logRecord := range scopeLog.LogRecords {
		s.processLogRecord(ctx, logRecord)
	}
}

func (s *LogServer) processLogRecord(ctx context.Context, logRecord *v12.LogRecord) {
	var jsonData *v1.AnyValue
	for _, kv := range logRecord.Body.GetKvlistValue().GetValues() {
		if kv.GetKey() == "JSON" {
			jsonData = kv.GetValue()
			break
		}
	}
	if jsonData == nil || jsonData.GetStringValue() == "" {
		return
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonData.GetStringValue()), &data); err != nil {
		s.logger.Error().Err(err).Msg("Error parsing JSON")
		return
	}

	if data["store_to_clickhouse"+logging.GetAbbreviation("bool")] == true {
		storeData(ctx, s.logger, s.connect, data)
	}
}

func (s *LogServer) Export(ctx context.Context, req *logs.ExportLogsServiceRequest) (*logs.ExportLogsServiceResponse, error) {
	s.logger.Info().Int("log_records", len(req.ResourceLogs)).Msg("Received log records")
	for _, resourceLog := range req.ResourceLogs {
		s.processResourceLog(ctx, resourceLog)
	}
	return &logs.ExportLogsServiceResponse{}, nil
}

func Run(ctx context.Context, cfg Config, logger zerolog.Logger) error {
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

	if err := createDatabaseIfNotExists(ctx, connect, DefaultDatabase); err != nil {
		logger.Error().Err(err).Msg("Error creating default database")
		return err
	}

	if err := createTableIfNotExists(ctx, connect, DefaultDatabase, DefaultTable); err != nil {
		logger.Error().Err(err).Msg("Error creating default table")
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

	go func() {
		if err := server.Serve(listener); err != nil {
			logger.Error().Err(err).Msg("Failed to serve gRPC server")
		}
	}()

	<-ctx.Done()
	logger.Info().Msg("Shutting down gRPC server due to context cancellation")

	server.GracefulStop()
	return nil
}
