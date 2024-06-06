package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteEthCall struct {
	suite.Suite
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
	contracts     map[string]*compiler.Contract
	from          types.Address
}

func (s *SuiteEthCall) SetupSuite() {
	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	tx, err := s.db.CreateRwTx(ctx)
	s.Require().NoError(err)

	s.lastBlockHash = common.EmptyHash
	es, err := execution.NewExecutionState(tx, shardId, s.lastBlockHash, common.NewTestTimer(0))
	s.Require().NoError(err)

	blockContext := execution.NewEVMBlockContext(es)

	s.contracts, err = solc.CompileSource("../../core/execution/testdata/call.sol")
	s.Require().NoError(err)

	s.from = types.GenerateRandomAddress(shardId)

	dm := &types.DeployMessage{
		ShardId: shardId,
		Code:    hexutil.FromHex(s.contracts["SimpleContract"].Code),
	}
	data, err := dm.MarshalSSZ()
	s.Require().NoError(err)

	m := &types.Message{Seqno: 0, Data: data, From: s.from, GasLimit: *types.NewUint256(100000)}
	es.AddInMessage(m)

	es.AddInMessage(m)
	s.Require().NoError(es.HandleDeployMessage(m, dm, &blockContext))

	blockHash, err := es.Commit(0)
	s.Require().NoError(err)
	s.lastBlockHash = blockHash

	block, err := execution.PostprocessBlock(tx, shardId, blockHash)
	s.Require().NotNil(block)
	s.Require().NoError(err)

	err = tx.Commit()
	s.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	s.Require().NotNil(pool)

	s.api = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), s.db, []msgpool.Pool{pool}, common.NewLogger("Test"))
}

func (s *SuiteEthCall) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthCall) TestSmcCall() {
	ctx := context.Background()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	calldata, err := abi.Pack("getValue")
	s.Require().NoError(err)

	to := types.CreateAddress(types.BaseShardId, s.from, 0)
	callArgsData := hexutil.Bytes(calldata)
	args := CallArgs{
		From:     s.from,
		Data:     callArgsData,
		To:       to,
		Value:    types.Uint256{Int: *uint256.NewInt(0)},
		GasLimit: types.Uint256{Int: *uint256.NewInt(10000)},
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
	args.GasLimit = types.Uint256{}
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
