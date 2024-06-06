package execution

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteZeroState struct {
	suite.Suite

	faucetAddr types.Address
	faucetABI  abi.ABI

	state        *ExecutionState
	blockContext vm.BlockContext
	contracts    map[string]*compiler.Contract
}

func (suite *SuiteZeroState) SetupSuite() {
	pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
	suite.faucetAddr = types.PubkeyBytesToAddress(types.BaseShardId, pub)

	contractsPath, err := obtainContractsPath()
	suite.Require().NoError(err)
	contracts, err := solc.CompileSource(contractsPath)
	suite.Require().NoError(err)
	suite.faucetABI = solc.ExtractABI(contracts["Faucet"])
}

func (suite *SuiteZeroState) SetupTest() {
	suite.state = newState(suite.T())
	suite.blockContext = NewEVMBlockContext(suite.state)

	var err error
	suite.contracts, err = solc.CompileSource("./testdata/call.sol")
	suite.Require().NoError(err)
}

func (suite *SuiteZeroState) TearDownTest() {
}

func (suite *SuiteZeroState) getBalance(address types.Address) uint256.Int {
	suite.T().Helper()

	account, ok := suite.state.Accounts[address]
	suite.Require().True(ok)
	return account.Balance
}

func (suite *SuiteZeroState) TestFaucetBalance() {
	ret := suite.getBalance(suite.faucetAddr)
	suite.Require().EqualValues(*uint256.NewInt(1000000000000), ret)
}

func (suite *SuiteZeroState) TestWithdrawFromFaucet() {
	receiverContract := suite.contracts["SimpleContract"]
	receiverAddr, err := deployContract(receiverContract, suite.state, &suite.blockContext, 2)
	suite.Require().NoError(err)

	calldata, err := suite.faucetABI.Pack("withdrawTo", receiverAddr, big.NewInt(100))
	suite.Require().NoError(err)

	callMessage := &types.Message{
		Data:     calldata,
		To:       suite.faucetAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, err = suite.state.HandleExecutionMessage(callMessage, &suite.blockContext)
	suite.Require().NoError(err)

	suite.Require().EqualValues(*uint256.NewInt(1000000000000 - 100), suite.getBalance(suite.faucetAddr))
	suite.Require().EqualValues(*uint256.NewInt(100), suite.getBalance(receiverAddr))
}

func TestSuiteZeroState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteZeroState))
}
