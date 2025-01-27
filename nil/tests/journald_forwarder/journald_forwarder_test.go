package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/journald_forwarder"
	"github.com/stretchr/testify/suite"
	logs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SuiteJournaldForwarder struct {
	suite.Suite
	context    context.Context
	ctxCancel  context.CancelFunc
	cfg        journald_forwarder.Config
	clickhouse *exec.Cmd
	wg         sync.WaitGroup
	runErrCh   chan error
}

func (s *SuiteJournaldForwarder) SetupSuite() {
	suiteSetupDone := false

	s.context, s.ctxCancel = context.WithCancel(context.Background())
	defer func() {
		if !suiteSetupDone {
			s.TearDownSuite()
		}
	}()

	dir := s.T().TempDir()
	s.clickhouse = exec.Command( //nolint:gosec
		"clickhouse", "server", "--",
		"--tcp_port=9001",
		"--path="+dir,
	)
	s.clickhouse.Dir = dir
	err := s.clickhouse.Start()
	s.Require().NoError(err)

	s.cfg = journald_forwarder.Config{
		ListenAddr: "127.0.0.1:5678", ClickhouseAddr: "127.0.0.1:9001", DbUser: "default",
		DbDatabase: "default", DbPassword: "",
	}
	s.runErrCh = make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := journald_forwarder.Run(s.context, s.cfg, logging.NewLogger("test_journald_forwarder")); err != nil {
			s.runErrCh <- err
		} else {
			s.runErrCh <- nil
		}
	}()
	time.Sleep(time.Second)

	suiteSetupDone = true
}

func (s *SuiteJournaldForwarder) TearDownSuite() {
	s.ctxCancel()
	s.wg.Wait()
	if s.clickhouse != nil {
		err := s.clickhouse.Process.Kill()
		s.Require().NoError(err)
	}
}

func (s *SuiteJournaldForwarder) getTableValues(connect driver.Conn, database, table string) []map[string]any {
	s.T().Helper()
	query := fmt.Sprintf("SELECT * FROM %s.%s;", database, table)

	schema := s.getTableSchema(connect, database, table)

	rows, err := connect.Query(context.Background(), query)
	s.Require().NoError(err)

	defer rows.Close()

	columns := rows.Columns()
	s.Require().NotEmpty(columns)

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		for i, col := range columns {
			switch schema[col] {
			case "DateTime":
				values[i] = new(time.Time)
			case "UInt8":
				values[i] = new(bool)
			case "Int64":
				values[i] = new(int64)
			case "UInt256":
				values[i] = new(big.Int)
			case "Float64":
				values[i] = new(float64)
			case "String":
				values[i] = new(string)
			default:
				values[i] = new(any)
			}
		}
		s.Require().NoError(rows.Scan(values...))

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = dereference(values[i])
		}

		results = append(results, row)
	}
	s.Require().NoError(rows.Err())

	return results
}

func dereference(value any) any {
	switch v := value.(type) {
	case *string:
		return *v
	case *int64:
		return *v
	case *big.Int:
		return v.String()
	case *bool:
		return *v
	case *float64:
		return *v
	case *time.Time:
		return v.Format("2006-01-02T15:04:05Z") // Format DateTime as ISO8601 string
	default:
		return v
	}
}

func (s *SuiteJournaldForwarder) getTableSchema(connect driver.Conn, database, table string) map[string]string {
	s.T().Helper()
	query := fmt.Sprintf(
		"SELECT name, type FROM system.columns WHERE database = '%s' AND table = '%s';",
		database, table,
	)

	rows, err := connect.Query(context.Background(), query)
	s.Require().NoError(err)

	defer rows.Close()

	schema := make(map[string]string)
	for rows.Next() {
		var columnName, columnType string
		s.Require().NoError(rows.Scan(&columnName, &columnType))
		schema[columnName] = columnType
	}

	return schema
}

func (s *SuiteJournaldForwarder) dropDatabase(connect clickhouse.Conn, dbName string) {
	s.T().Helper()
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s;", dbName)
	s.Require().NoError(connect.Exec(context.Background(), query))
}

func (s *SuiteJournaldForwarder) TestLogDataInsert() {
	connect, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.cfg.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: s.cfg.DbUser,
			Password: "",
		},
	})
	s.Require().NoErrorf(err, "Failed to connect to ClickHouse")
	defer connect.Close()

	database := "x1"
	table := "x2"
	s.dropDatabase(connect, database)

	valueUInt256, _ := new(big.Int).SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)
	metrics := map[string]any{
		"valueFloat":   123.01,
		"valueStr":     "test log",
		"valueBool":    false,
		"valueDate":    "2023-12-17T10:30:00Z",
		"valueInt64":   int64(123456789012345),
		"valueUInt256": valueUInt256.String(),
	}
	data := map[string]any{
		"store_to_clickhouse": true,
		"database":            database,
		"table":               table,
	}
	dataNoSend1 := map[string]any{
		"store_to_clickhouse": false,
		"database":            database,
		"table":               table,
	}
	dataNoSend2 := map[string]any{
		"database": database,
		"table":    table,
	}
	dataArray := []map[string]any{data, dataNoSend1, dataNoSend2}
	dataStringArray := make([]string, 0, len(dataArray))

	for _, curData := range dataArray {
		for key, value := range metrics {
			curData[key] = value
		}
		dataString, err := json.Marshal(curData)
		if err != nil {
			fmt.Printf("Error converting map to JSON: %v\n", err)
			return
		}
		dataStringArray = append(dataStringArray, string(dataString))
	}
	dataType := map[string]any{
		"valueFloat":   "Float64",
		"valueStr":     "String",
		"valueBool":    "UInt8",
		"valueDate":    "DateTime",
		"valueInt64":   "Int64",
		"valueUInt256": "UInt256",
	}
	for _, dataString := range dataStringArray {
		s.Require().NoError(sendLogs(s.cfg.ListenAddr, dataString))
	}
	time.Sleep(1 * time.Second)
	schema := s.getTableSchema(connect, database, table)
	s.Require().NoError(err)
	for key, value := range dataType {
		s.Require().Contains(schema, key)
		s.Require().Equal(value, schema[key])
	}
	values := s.getTableValues(connect, database, table)
	s.Require().NoError(err)
	s.Require().Len(values, 1)
	s.Require().Equal(metrics, values[0])
}

func createLogRecord(key, value string) *v1.LogRecord {
	return &v1.LogRecord{
		Body: &common.AnyValue{
			Value: &common.AnyValue_KvlistValue{
				KvlistValue: &common.KeyValueList{
					Values: []*common.KeyValue{
						{
							Key: key,
							Value: &common.AnyValue{
								Value: &common.AnyValue_StringValue{
									StringValue: value,
								},
							},
						},
					},
				},
			},
		},
	}
}

func createScopeLogs(logRecord *v1.LogRecord) *v1.ScopeLogs {
	return &v1.ScopeLogs{
		LogRecords: []*v1.LogRecord{logRecord},
	}
}

func createResourceLogs(scopeLogs *v1.ScopeLogs) *v1.ResourceLogs {
	return &v1.ResourceLogs{
		ScopeLogs: []*v1.ScopeLogs{scopeLogs},
	}
}

func sendLogs(listenAddress string, dataString string) error {
	conn, err := grpc.NewClient(listenAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := logs.NewLogsServiceClient(conn)

	logRecord := &logs.ExportLogsServiceRequest{
		ResourceLogs: []*v1.ResourceLogs{
			createResourceLogs(
				createScopeLogs(
					createLogRecord("JSON", dataString),
				),
			),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Export(ctx, logRecord)
	return err
}

func checkClickhouseInstalled() bool {
	cmd := exec.Command("clickhouse", "--version")
	err := cmd.Run()
	return err == nil
}

func TestJournaldForwarderClickhouse(t *testing.T) {
	if !checkClickhouseInstalled() {
		if assert.Enable {
			t.Fatal("Clickhouse is not installed")
		} else {
			t.Skip("Clickhouse is not installed")
		}
	}
	t.Parallel()
	suite.Run(t, new(SuiteJournaldForwarder))
}
