package jsonrpc

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
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
	ctx := s.T().Context()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.contracts, err = solc.CompileSource("../../../internal/execution/testdata/call.sol")
	s.Require().NoError(err)

	mainBlock := execution.GenerateZeroState(s.T(), types.MainShardId, s.db)

	m1 := execution.NewDeployTransaction(
		types.BuildDeployPayload(hexutil.FromHex(s.contracts["Caller"].Code), common.EmptyHash),
		shardId,
		types.GenerateRandomAddress(shardId),
		0,
		types.GasToValue(100_000_000))

	s.from = m1.To

	m2 := execution.NewDeployTransaction(
		types.BuildDeployPayload(hexutil.FromHex(s.contracts["SimpleContract"].Code), common.EmptyHash),
		shardId,
		s.from,
		0,
		types.Value{})

	s.simple = m2.To

	s.lastBlockHash = execution.GenerateBlockFromTransactions(
		s.T(), ctx, shardId, 0, s.lastBlockHash, s.db, nil, m1, m2)

	execution.GenerateBlockFromTransactions(
		s.T(),
		ctx,
		types.MainShardId,
		0,
		mainBlock.Hash(types.MainShardId),
		s.db,
		map[types.ShardId]common.Hash{shardId: s.lastBlockHash})

	s.api = NewTestEthAPI(s.T(), ctx, s.db, 2)
}

func (s *SuiteEthCall) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthCall) TestSmcCall() {
	ctx := s.T().Context()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	calldata, err := abi.Pack("getValue")
	s.Require().NoError(err)

	to := s.simple
	callArgsData := hexutil.Bytes(calldata)
	args := CallArgs{
		Flags: types.NewTransactionFlags(types.TransactionFlagInternal),
		From:  &s.from,
		Data:  &callArgsData,
		To:    to,
		Fee:   types.NewFeePackFromGas(10_000),
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
	args.Fee = types.NewFeePackFromGas(1)
	res, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num}, nil)
	s.Require().NoError(err)
	s.Require().Contains(res.Error, vm.ErrOutOfGas.Error())

	// Unknown "to"
	args.Fee = types.NewFeePackFromGas(10_000)
	args.To = types.GenerateRandomAddress(types.BaseShardId)
	res, err = s.api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &num}, nil)
	s.Require().NoError(err)
	s.Require().Empty(res.Error)
	s.Require().Empty(res.Data)
}

func (s *SuiteEthCall) TestChainCall() {
	ctx := s.T().Context()

	abi := solc.ExtractABI(s.contracts["SimpleContract"])
	getCalldata, err := abi.Pack("getValue")
	s.Require().NoError(err)

	setCalldata, err := abi.Pack("setValue", big.NewInt(123))
	s.Require().NoError(err)

	to := s.simple
	getData := hexutil.Bytes(getCalldata)
	setData := hexutil.Bytes(setCalldata)
	args := CallArgs{
		Data: &getData,
		To:   to,
		Fee:  types.NewFeePackFromGas(10_000),
	}
	res, err := s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().NoError(err)
	s.EqualValues(0x2a, s.unpackGetValue(res.Data))

	args.Data = &setData
	res, err = s.api.Call(ctx, args, latestBlockId, nil)
	s.Require().NoError(err)
	s.Empty(res.Data)
	s.Len(res.StateOverrides, 1)
	s.NotNil(res.StateOverrides[s.simple].StateDiff)
	s.Nil(res.StateOverrides[s.simple].Balance)

	args.Data = &getData
	res, err = s.api.Call(ctx, args, latestBlockId, &res.StateOverrides)
	s.Require().NoError(err)
	s.EqualValues(0x7b, s.unpackGetValue(res.Data))
}

func TestSuiteEthCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthCall))
}
