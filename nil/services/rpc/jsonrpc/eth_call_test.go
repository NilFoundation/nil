package jsonrpc

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/rpc/transport/rpccfg"
	"github.com/NilFoundation/nil/nil/tools/solc"
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

var latestBlockId = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}

func (s *SuiteEthCall) unpackGetValue(data []byte) uint64 {
	s.T().Helper()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	res, err := abi.Unpack("getValue", data)
	s.Require().NoError(err)
	v, ok := res[0].(*big.Int)
	s.Require().True(ok)
	return v.Uint64()
}

func (s *SuiteEthCall) SetupSuite() {
	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.contracts, err = solc.CompileSource("../../../internal/execution/testdata/call.sol")
	s.Require().NoError(err)

	s.from = types.GenerateRandomAddress(shardId)

	m := execution.NewDeployMessage(types.BuildDeployPayload(hexutil.FromHex(s.contracts["SimpleContract"].Code), common.EmptyHash),
		shardId, s.from, 0)
	s.simple = m.To

	s.lastBlockHash = execution.GenerateBlockFromMessages(s.T(), ctx, shardId, 0, s.lastBlockHash, s.db, nil, m)

	execution.GenerateBlockFromMessages(s.T(), ctx, types.MainShardId, 0, common.EmptyHash, s.db,
		map[types.ShardId]common.Hash{shardId: s.lastBlockHash})

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
		From:      &s.from,
		Data:      callArgsData,
		To:        to,
		FeeCredit: types.GasToValue(10_000),
	}
	res, err := s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().NoError(err)
	s.EqualValues(0x2a, s.unpackGetValue(res.Data))

	// Call with block number
	num := transport.BlockNumber(0)
	res, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num}, nil)
	s.Require().NoError(err)
	s.EqualValues(0x2a, s.unpackGetValue(res.Data))

	// Out of gas
	args.FeeCredit = types.GasToValue(0)
	_, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num}, nil)
	s.Require().ErrorIs(err, vm.ErrOutOfGas)

	// Call with invalid arguments
	payload := types.BuildDeployPayload(common.EmptyHash[:], common.EmptyHash)
	args.To = types.CreateAddress(0, payload)
	args.Data = payload.Bytes()
	res, err = s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().ErrorIs(err, execution.ErrDeployToMainShard)
	s.Require().Nil(res)
}

func (s *SuiteEthCall) TestChainCall() {
	ctx := context.Background()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	getCalldata, err := abi.Pack("getValue")
	s.Require().NoError(err)

	setCalldata, err := abi.Pack("setValue", big.NewInt(123))
	s.Require().NoError(err)

	to := s.simple
	callArgsData := hexutil.Bytes(getCalldata)
	args := CallArgs{
		From:      &s.from,
		Data:      callArgsData,
		To:        to,
		FeeCredit: types.GasToValue(10_000),
	}
	res, err := s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().NoError(err)
	s.EqualValues(0x2a, s.unpackGetValue(res.Data))

	args.Data = setCalldata
	res, err = s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().NoError(err)
	s.Empty(res.Data)
	s.Len(res.StateOverrides, 2)
	s.Nil(res.StateOverrides[s.from].StateDiff)
	s.NotNil(res.StateOverrides[s.from].Balance)

	s.NotNil(res.StateOverrides[s.simple].StateDiff)
	s.Nil(res.StateOverrides[s.simple].Balance)

	args.Data = getCalldata
	res, err = s.api.Call(ctx, args, latestBlockId, &res.StateOverrides)
	s.Require().NoError(err)
	s.EqualValues(0x7b, s.unpackGetValue(res.Data))
}

func TestSuiteEthCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthCall))
}
