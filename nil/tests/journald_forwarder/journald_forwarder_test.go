package main

import (
	"bytes"
	"context"
	"fmt"
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
	connect    driver.Conn
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
		"--http_port=",
		"--mysql_port=",
		"--path="+dir,
	)
	s.clickhouse.Dir = dir
	err := s.clickhouse.Start()
	s.Require().NoError(err)
	time.Sleep(time.Second)

	s.cfg = journald_forwarder.Config{
		ListenAddr: "127.0.0.1:5678", ClickhouseAddr: "127.0.0.1:9001", DbUser: "default",
		DbDatabase: "default", DbPassword: "",
	}

	s.connect, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.cfg.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: s.cfg.DbUser,
			Password: "",
		},
	})
	s.Require().NoErrorf(err, "Failed to connect to ClickHouse")
	defer s.connect.Close()
	s.dropDatabase(journald_forwarder.DefaultDatabase)

	s.runErrCh = make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := journald_forwarder.Run(s.context, s.cfg, logging.NewLogger("test_journald_forwarder", false).Logger()); err != nil {
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

func (s *SuiteJournaldForwarder) dropDatabase(dbName string) {
	s.T().Helper()
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s;", dbName)
	s.Require().NoError(s.connect.Exec(context.Background(), query))
}

func (s *SuiteJournaldForwarder) TestLogDataInsert() {
	s.Run("Check insert columns", func() {
		logBuf := new(bytes.Buffer)
		log1 := logging.NewLoggerWithWriter("log1", true, logBuf)
		logger := log1.Float64("valueFloat", 123.01).Str("valueStr", "test log").Logger()
		logger.Log().Msg("test log1")
		s.Require().NoError(sendLogs(s.cfg.ListenAddr, logBuf.String()))
		time.Sleep(1 * time.Second)
		schema1 := map[string]string{
			"time":       "DateTime",
			"message":    "String",
			"caller":     "String",
			"component":  "String",
			"valueFloat": "Float64",
			"valueStr":   "String",
		}
		schemaRes := s.getTableSchema(s.connect, journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable)
		s.Require().Equal(schema1, schemaRes)

		logBuf = new(bytes.Buffer)
		log1 = logging.NewLoggerWithWriter("log2", true, logBuf)
		logger = log1.Uint256("valueUInt256", "115792089237316195423570985008687907853269984665640564039457584007913129639935").Logger()
		logger.Log().Msg("test log2")
		s.Require().NoError(sendLogs(s.cfg.ListenAddr, logBuf.String()))
		time.Sleep(1 * time.Second)
		schema2 := schema1
		schema2["valueUInt256"] = "UInt256"
		schemaRes = s.getTableSchema(s.connect, journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable)
		s.Require().Equal(schema2, schemaRes)

		logBuf = new(bytes.Buffer)
		log1 = logging.NewLoggerWithWriter("log2noCh", false, logBuf)
		logger = log1.Bool("newBool", false).Logger()
		logger.Log().Msg("test log2notCh")
		s.Require().NoError(sendLogs(s.cfg.ListenAddr, logBuf.String()))
		time.Sleep(1 * time.Second)
		schemaRes = s.getTableSchema(s.connect, journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable)
		s.Require().Equal(schema2, schemaRes)
	})
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
