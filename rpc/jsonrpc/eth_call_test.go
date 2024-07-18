package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/stretchr/testify/suite"
)

type SuiteEthCall struct {
	suite.Suite
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
	contracts     map[string]*compiler.Contract
	from          types.Address
	simple        types.Address
}

func (s *SuiteEthCall) SetupSuite() {
	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.contracts, err = solc.CompileSource("../../core/execution/testdata/call.sol")
	s.Require().NoError(err)

	s.from = types.GenerateRandomAddress(shardId)

	m := execution.NewDeployMessage(types.BuildDeployPayload(hexutil.FromHex(s.contracts["SimpleContract"].Code), common.EmptyHash),
		shardId, s.from, 0)
	s.simple = m.To

	s.lastBlockHash = execution.GenerateBlockFromMessages(s.T(), ctx, shardId, 0, s.lastBlockHash, s.db, m)

	pool := msgpool.New(msgpool.DefaultConfig)
	s.Require().NotNil(pool)

	s.api, err = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), s.db, []msgpool.Pool{pool}, logging.NewLogger("Test"))
	s.Require().NoError(err)
}

func (s *SuiteEthCall) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthCall) TestSmcCall() {
	ctx := context.Background()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	calldata, err := abi.Pack("getValue")
	s.Require().NoError(err)

	to := s.simple
	callArgsData := hexutil.Bytes(calldata)
	args := CallArgs{
		From:      s.from,
		Data:      callArgsData,
		To:        to,
		FeeCredit: types.GasToValue(10_000),
	}
	data, err := s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockHash: &s.lastBlockHash})
	s.Require().NoError(err)
	s.Require().Equal(uint8(0x2a), data[len(data)-1])

	// Call with block number
	num := transport.BlockNumber(0)
	data, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num})
	s.Require().NoError(err)
	s.Require().Equal(uint8(0x2a), data[len(data)-1])

	// Out of gas
	args.FeeCredit = types.GasToValue(0)
	_, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num})
	s.Require().ErrorIs(err, vm.ErrOutOfGas)

	// Call with invalid arguments
	args.To = types.EmptyAddress
	args.Data = []byte{}
	data, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockHash: &s.lastBlockHash})
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func TestSuiteEthCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthCall))
}
