package cometa

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteCometa struct {
	tests.RpcSuite
	cometaClient   cometa.Client
	cometaCfg      cometa.Config
	cometaEndpoint string
}

type SuiteCometaBadger struct {
	SuiteCometa
}

type SuiteCometaClickhouse struct {
	SuiteCometa
	clickhouse *exec.Cmd
}

func (s *SuiteCometa) SetupSuite() {
	s.cometaCfg.DbPath = s.T().TempDir() + "/cometa.db"
	s.cometaCfg.OwnEndpoint = ""

	s.Start(&nilservice.Config{
		NShards:              2,
		CollatorTickPeriodMs: 200,
		HttpUrl:              rpc.GetSockPath(s.T()),
		Cometa:               &s.cometaCfg,
	})

	s.cometaEndpoint = rpc.GetSockPathService(s.T(), "cometa")

	s.cometaClient = *cometa.NewClient(s.Endpoint)
}

func (s *SuiteCometaClickhouse) SetupSuite() {
	s.cometaCfg.UseBadger = false

	s.cometaCfg.ResetToDefault()

	suiteSetupDone := false

	defer func() {
		if !suiteSetupDone {
			s.TearDownSuite()
		}
	}()

	s.clickhouse = exec.Command("clickhouse", "server", "--", "--listen_host=0.0.0.0")
	s.clickhouse.Dir = s.T().TempDir()
	err := s.clickhouse.Start()
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	createDb := exec.Command("clickhouse-client", "--query", "CREATE DATABASE IF NOT EXISTS "+s.cometaCfg.DbName) //nolint:gosec
	out, err := createDb.CombinedOutput()
	s.Require().NoError(err, fmt.Sprintf("output: %s", out))

	s.SuiteCometa.SetupSuite()

	suiteSetupDone = true
}

func (s *SuiteCometaClickhouse) TearDownSuite() {
	if s.clickhouse != nil {
		err := s.clickhouse.Process.Kill()
		s.Require().NoError(err)
	}
}

func (s *SuiteCometaBadger) SetupSuite() {
	s.cometaCfg.ResetToDefault()
	s.cometaCfg.UseBadger = true
	s.SuiteCometa.SetupSuite()
}

func (s *SuiteCometa) TestTwinContracts() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)
	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode1 := types.BuildDeployPayload(walletCode, common.EmptyHash)
	deployCode2 := types.BuildDeployPayload(walletCode, common.HexToHash("0x1234"))

	walletAddr1, _ := s.DeployContractViaMainWallet(types.BaseShardId, deployCode1, types.NewValueFromUint64(10_000_000))
	walletAddr2, _ := s.DeployContractViaMainWallet(types.BaseShardId, deployCode2, types.NewValueFromUint64(10_000_000))

	err = s.cometaClient.RegisterContractFromFile("../../contracts/solidity/compile-wallet.json", walletAddr1)
	s.Require().NoError(err)

	contract1, err := s.cometaClient.GetContract(walletAddr1)
	s.Require().NoError(err)

	contract2, err := s.cometaClient.GetContract(walletAddr2)
	s.Require().NoError(err)

	s.Require().Equal(contract1, contract2)
}

func checkClickhouseInstalled() bool {
	cmd := exec.Command("clickhouse", "--version")
	err := cmd.Run()
	return err == nil
}

func TestCometaClickhouse(t *testing.T) {
	if !checkClickhouseInstalled() {
		if assert.Enable {
			t.Fatal("Clickhouse is not installed")
		} else {
			t.Skip("Clickhouse is not installed")
		}
	}
	t.Parallel()
	suite.Run(t, new(SuiteCometaClickhouse))
}

func TestCometaBadger(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SuiteCometaBadger))
}
