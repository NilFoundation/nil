package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/journald_forwarder"
	"github.com/NilFoundation/nil/nil/services/rpc"
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
	cfg        journald_forwarder.ClickhouseConfig
	clickhouse *exec.Cmd
	connection driver.Conn
	wg         sync.WaitGroup
	runErrCh   chan error
}

func (s *SuiteJournaldForwarder) SetupSuite() {
	suiteSetupDone := false

	defer func() {
		if !suiteSetupDone {
			s.TearDownSuite()
		}
	}()

	dir := s.T().TempDir() + "/clickhouse"
	s.Require().NoError(os.MkdirAll(dir, 0o755))
	s.clickhouse = exec.Command( //nolint:gosec
		"clickhouse", "server", "--",
		"--tcp_port=9001",
		"--http_port=",
		"--mysql_port=",
		"--path="+dir,
	)
	s.clickhouse.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	s.clickhouse.Dir = dir
	err := s.clickhouse.Start()
	s.Require().NoError(err)
	time.Sleep(time.Second)

	socketPath := rpc.GetSockPath(s.T())
	socketPathDir := strings.ReplaceAll(socketPath, "unix://", "")
	s.Require().NoError(os.MkdirAll(filepath.Dir(socketPathDir), 0o755))

	s.cfg = journald_forwarder.ClickhouseConfig{
		ListenAddr: socketPath, ClickhouseAddr: "127.0.0.1:9001", DbUser: "default",
		DbDatabase: "default", DbPassword: "",
	}

	s.connection, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.cfg.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: s.cfg.DbDatabase,
			Username: s.cfg.DbUser,
			Password: "",
		},
	})
	s.Require().NoError(err, "Failed to connection to ClickHouse")

	suiteSetupDone = true
}

func (s *SuiteJournaldForwarder) TearDownSuite() {
	if s.connection != nil {
		s.Require().NoError(s.connection.Close())
	}

	if s.clickhouse != nil {
		// https://stackoverflow.com/questions/22470193/why-wont-go-kill-a-child-process-correctly
		// simple s.clickhouse.Kill() won't work on child process
		// this leads to errors in sequential test runs
		pgid, err := syscall.Getpgid(s.clickhouse.Process.Pid)
		s.Require().NoError(err)
		s.Require().NoError(syscall.Kill(-pgid, syscall.SIGKILL))
	}
}

func (s *SuiteJournaldForwarder) SetupTest() {
	s.context, s.ctxCancel = context.WithCancel(context.Background())

	s.dropDatabase(journald_forwarder.DefaultDatabase)

	s.runErrCh = make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runErrCh <- journald_forwarder.Run(
			s.context, s.cfg, logging.NewLoggerWithStore("test_journald_forwarder", false))
	}()
	time.Sleep(time.Second)
}

func (s *SuiteJournaldForwarder) TearDownTest() {
	s.ctxCancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.T().Log("TearDownTest timeout! Possible deadlock.")
	}

	select {
	case err := <-s.runErrCh:
		if err != nil {
			s.T().Logf("Error from journald_forwarder.Run: %v", err)
		}
	default:
	}
}

func (s *SuiteJournaldForwarder) getTableSchema(connect driver.Conn) map[string]string {
	s.T().Helper()
	query := fmt.Sprintf(
		"SELECT name, type FROM system.columns WHERE database = '%s' AND table = '%s';",
		journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable,
	)

	rows, err := connect.Query(s.context, query)
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
	s.Require().NoError(s.connection.Exec(s.context, query))
}

func (s *SuiteJournaldForwarder) TestLogDataInsert() {
	valueString := "test log"
	valueFloat := 123.01
	valueMessage := "test log1"

	logBuf := new(bytes.Buffer)
	logger := logging.NewLoggerWithWriter("log1", logBuf).With().
		Float64("valueFloat", valueFloat).Str("valueStr", valueString).Logger()
	logger.Info().Err(errors.New("test error")).Msg(valueMessage)

	s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf.String()))
	time.Sleep(1 * time.Second)

	schema1 := map[string]string{
		"_HOSTNAME":     "String",
		"_SYSTEMD_UNIT": "String",
		"time":          "DateTime64(3)",
		"level":         "String",
		"error":         "String",
		"message":       "String",
		"caller":        "String",
		"component":     "String",
		"valueFloat":    "Float64",
		"valueStr":      "String",
	}
	schemaRes := s.getTableSchema(s.connection)
	s.Require().Equal(schema1, schemaRes)

	query := fmt.Sprintf(
		"SELECT component, valueStr, valueFloat FROM %s.%s WHERE message = '%s';",
		journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable, valueMessage,
	)
	rows, err := s.connection.Query(s.context, query)
	s.Require().NoError(err)
	defer rows.Close()

	s.Require().True(rows.Next())

	var resComponent, resValueStr string
	var resValueFloat float64
	s.Require().NoError(rows.Scan(&resComponent, &resValueStr, &resValueFloat))

	s.Require().Equal("log1", resComponent)
	s.Require().Equal(valueString, resValueStr)
	s.Require().InEpsilon(valueFloat, resValueFloat, 0.0001)

	logBuf = new(bytes.Buffer)
	logger = logging.NewLoggerWithWriter("log2", logBuf).With().
		Uint256(
			"valueUInt256",
			"115792089237316195423570985008687907853269984665640564039457584007913129639935").
		Logger()
	logger.Log().Msg("test log2")
	s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf.String()))
	time.Sleep(1 * time.Second)
	schema2 := schema1
	schema2["valueUInt256"] = "UInt256"
	schemaRes = s.getTableSchema(s.connection)
	s.Require().Equal(schema2, schemaRes)

	logBuf = new(bytes.Buffer)
	logger = logging.NewLoggerWithWriterStore("log2noCh", false, logBuf).With().Bool("newBool", false).Logger()
	logger.Log().Msg("test log2notCh")
	s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf.String()))
	time.Sleep(1 * time.Second)
	schemaRes = s.getTableSchema(s.connection)
	s.Require().Equal(schema2, schemaRes)
}

func (s *SuiteJournaldForwarder) TestBatchLogDataInsert() {
	s.Run("Check batch insert with same columns", func() {
		logBuf1 := new(bytes.Buffer)
		logger1 := logging.NewLoggerWithWriter("log", logBuf1).With().
			Int("x1", 1).Logger()
		logBuf2 := new(bytes.Buffer)
		logger2 := logging.NewLoggerWithWriter("log", logBuf2).With().
			Int("x1", 1).Logger()

		logger1.Info().Msg("")
		logger2.Info().Msg("")
		s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf1.String(), logBuf2.String()))
		time.Sleep(1 * time.Second)

		schema := map[string]string{
			"_HOSTNAME":     "String",
			"_SYSTEMD_UNIT": "String",
			"time":          "DateTime64(3)",
			"level":         "String",
			"caller":        "String",
			"component":     "String",
			"x1":            "Int64",
		}
		schemaRes := s.getTableSchema(s.connection)
		s.Require().Equal(schema, schemaRes)
	})

	s.Run("Check batch insert with different columns", func() {
		logBuf1 := new(bytes.Buffer)
		logger1 := logging.NewLoggerWithWriter("log", logBuf1).With().
			Int("x1", 1).Logger()
		logBuf2 := new(bytes.Buffer)
		logger2 := logging.NewLoggerWithWriter("log", logBuf2).With().
			Int("x1", 1).Str("z1", "hello").Int("z2", 2).Logger()

		logger1.Info().Msg("")
		logger2.Info().Msg("")

		s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf1.String()))
		s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf1.String(), logBuf2.String()))
		time.Sleep(1 * time.Second)

		schema := map[string]string{
			"_HOSTNAME":     "String",
			"_SYSTEMD_UNIT": "String",
			"time":          "DateTime64(3)",
			"level":         "String",
			"caller":        "String",
			"component":     "String",
			"x1":            "Int64",
			"z1":            "String",
			"z2":            "Int64",
		}
		schemaRes := s.getTableSchema(s.connection)
		s.Require().Equal(schema, schemaRes)
	})
}

func (s *SuiteJournaldForwarder) TestInsertedValue() {
	x1 := 1
	x2 := 2
	x3 := 3
	logBuf1 := new(bytes.Buffer)
	logger1 := logging.NewLoggerWithWriter("log", logBuf1).With().
		Int("x1", x1).Str("record", "first").Logger()

	logBuf2 := new(bytes.Buffer)
	logger2 := logging.NewLoggerWithWriter("log", logBuf2).With().
		Int("x2", x2).Int("x3", x3).Str("record", "second").Logger()

	logger1.Info().Msg("")
	logger2.Info().Msg("")
	s.Require().NoError(sendLogs(s.context, s.cfg.ListenAddr, logBuf1.String(), logBuf2.String()))
	time.Sleep(1 * time.Second)

	query := fmt.Sprintf(
		"SELECT record, x1, x2, x3 FROM %s.%s;",
		journald_forwarder.DefaultDatabase, journald_forwarder.DefaultTable,
	)
	rows, err := s.connection.Query(s.context, query)
	s.Require().NoError(err)
	defer rows.Close()

	var tableX1, tableX2, tableX3 int64
	var st string
	for rows.Next() {
		s.Require().NoError(rows.Scan(&st, &tableX1, &tableX2, &tableX3))

		if st == "first" {
			s.Require().Equal(int64(x1), tableX1)
			s.Require().Equal(int64(0), tableX2)
			s.Require().Equal(int64(0), tableX3)
			continue
		}
		if st == "second" {
			s.Require().Equal(int64(0), tableX1)
			s.Require().Equal(int64(x2), tableX2)
			s.Require().Equal(int64(x3), tableX3)
			continue
		}

		s.Require().Failf("unexpected value for string variable: %s", st)
	}
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

func sendLogs(ctx context.Context, listenAddress string, dataStrings ...string) error {
	conn, err := grpc.NewClient(listenAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := logs.NewLogsServiceClient(conn)

	resourceLogs := make([]*v1.ResourceLogs, 0, len(dataStrings))
	for _, data := range dataStrings {
		resourceLogs = append(resourceLogs, createResourceLogs(createScopeLogs(createLogRecord("JSON", data))))
	}

	logRecord := &logs.ExportLogsServiceRequest{
		ResourceLogs: resourceLogs,
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
