package clickhouse

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteClickhouse struct {
	suite.Suite
	clickhouse *exec.Cmd
	driver     *ClickhouseDriver
}

func (s *SuiteClickhouse) SetupSuite() {
	dir := s.T().TempDir()
	s.clickhouse = exec.Command( //nolint:gosec
		"clickhouse", "server", "--",
		"--listen_host=127.0.0.1",
		"--tcp_port=9003",
		"--http_port=8123",
		"--mysql_port=",
		"--path="+dir,
	)
	s.clickhouse.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	s.clickhouse.Dir = dir
	err := s.clickhouse.Start()
	s.Require().NoError(err)

	s.Require().Eventually(func() bool {
		resp, err := http.Get("http://127.0.0.1:8123/ping")
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}

		return string(body) == "Ok.\n"
	}, 30*time.Second, 500*time.Millisecond, "ClickHouse server did not start within the allocated time")

	s.driver, err = NewClickhouseDriver(context.Background(), "127.0.0.1:9003", "", "", "nil_database")
	s.Require().NoError(err)
	s.Require().NotNil(s.driver)

	err = setupSchemes(s.T().Context(), s.driver.conn)
	s.Require().NoError(err)
}

func (s *SuiteClickhouse) TearDownSuite() {
	if s.clickhouse != nil {
		// https://stackoverflow.com/questions/22470193/why-wont-go-kill-a-child-process-correctly
		// simple s.clickhouse.Kill() won't work on child process
		// this leads to errors in sequential test runs
		pgid, err := syscall.Getpgid(s.clickhouse.Process.Pid)
		s.Require().NoError(err)
		err = syscall.Kill(-pgid, syscall.SIGTERM)
		s.Require().NoError(err)
	}
}

// This test should catch cases when new fields are added to the Transaction without supporting them in the driver.
func (s *SuiteClickhouse) TestTransactionWithBinaryBatching() {
	transactionBatch, err := s.driver.conn.PrepareBatch(s.T().Context(), "INSERT INTO transactions")
	s.Require().NoError(err)

	tx := types.NewEmptyTransaction()
	tokenId := types.TokenIdForAddress(types.Address{})
	tx.Token = []types.TokenBalance{{*tokenId, types.NewValueFromUint64(123)}}
	tx.RequestChain = []*types.AsyncRequestInfo{
		{
			Id:     123,
			Caller: types.MainSmartAccountAddress,
		},
	}

	txn := &TransactionWithBinary{Transaction: *tx}

	err = transactionBatch.AppendStruct(txn)
	s.Require().NoError(err)
}

func TestClickhouse(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SuiteClickhouse))
}
