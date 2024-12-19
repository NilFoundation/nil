package journald_forwarder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/rs/zerolog"
	logs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	v1 "go.opentelemetry.io/proto/otlp/common/v1"
	v12 "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
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

func isInt64(value string) bool {
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}

	if floatVal < math.MinInt64 || floatVal > math.MaxInt64 {
		return false
	}

	if floatVal != math.Trunc(floatVal) {
		return false
	}

	return true
}

func isUint256(value string) bool {
	num := new(big.Int)
	_, ok := num.SetString(value, 10)
	if !ok {
		return false
	}

	uint256Max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	return num.Sign() >= 0 && num.Cmp(uint256Max) <= 0
}

func determineType(value string) string {
	if value == "true" || value == "false" {
		return "UInt8" // ClickHouse uses UInt8 for boolean values
	}

	if isInt64(value) {
		return "Int64"
	}

	if isUint256(value) {
		return "UInt256"
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "Float64"
	}

	if _, err := time.Parse("2006-01-02T15:04:05Z", value); err == nil {
		return "DateTime"
	}

	return "String"
}

func createDatabaseIfNotExists(ctx context.Context, connect driver.Conn, database string) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", database)
	return connect.Exec(ctx, query)
}

func createTableIfNotExists(ctx context.Context, connect driver.Conn, database, tableName string, columns map[string]string, orderBy string) error {
	columnDefinitions := make([]string, 0, len(columns))
	for column, colType := range columns {
		columnDefinitions = append(columnDefinitions, fmt.Sprintf("%s %s", column, colType))
	}

	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (%s) ENGINE = MergeTree() ORDER BY %s;",
		database,
		tableName,
		strings.Join(columnDefinitions, ", "),
		orderBy,
	)

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

func getDatabaseAndTable(data map[string]any) (string, string) {
	db, ok := data["database"].(string)
	if !ok || db == "" {
		db = "default"
	}

	table, ok := data["table"].(string)
	if !ok || table == "" {
		table = "default"
	}

	return db, table
}

func storeData(ctx context.Context, logger zerolog.Logger, connect driver.Conn, data map[string]any) {
	db, table := getDatabaseAndTable(data)

	delete(data, "store_to_clickhouse")
	delete(data, "database")
	delete(data, "table")

	columns := []string{}
	columnsDef := map[string]string{}
	var values []any
	for key, value := range data {
		columns = append(columns, key)
		columnsDef[key] = determineType(fmt.Sprintf("%v", value))
		if columnsDef[key] == "DateTime" {
			parsedTime, err := time.Parse(time.RFC3339, value.(string))
			if err != nil {
				logger.Error().Err(err).Msg("Error parsing time")
				return
			}
			values = append(values, parsedTime.Format("2006-01-02 15:04:05"))
			continue
		} else {
			values = append(values, value)
		}
	}

	if err := createDatabaseIfNotExists(ctx, connect, db); err != nil {
		logger.Error().Err(err).Msg("Error creating database")
		return
	}

	if err := createTableIfNotExists(ctx, connect, db, table, columnsDef, columns[0]); err != nil {
		logger.Error().Err(err).Msg("Error creating table")
		return
	}

	if err := insertData(ctx, connect, db, table, columns, values); err != nil {
		logger.Error().Err(err).Msg("Error inserting data")
		return
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
	if jsonData == nil {
		return
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonData.GetStringValue()), &data); err != nil {
		s.logger.Error().Err(err).Msg("Error parsing JSON")
		return
	}

	if data["store_to_clickhouse"] == true {
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
