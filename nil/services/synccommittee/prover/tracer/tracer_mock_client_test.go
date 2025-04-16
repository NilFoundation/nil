package tracer

import (
	"context"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type TracerMockClientTestSuite struct {
	suite.Suite

	rpcClient    api.RpcClient
	shardId      types.ShardId
	accounts     map[types.Address]types.Code
	smartAccount types.Address
	inMsgs       []*types.Transaction
}

type dummyMptTracer struct {
	accountCodes map[types.Address]types.Code
	shardId      types.ShardId
	rwTx         db.RwTx
}

var _ execution.IContractMPTRepository = (*dummyMptTracer)(nil)

func newDummyMptTracer(
	accountCodes map[types.Address]types.Code,
	shardId types.ShardId,
	rwTx db.RwTx,
) (*dummyMptTracer, error) {
	for _, code := range accountCodes {
		if err := db.WriteCode(rwTx, shardId, code.Hash(), code); err != nil {
			return nil, err
		}
	}
	return &dummyMptTracer{
		accountCodes,
		shardId,
		rwTx,
	}, nil
}

func (t *dummyMptTracer) SetRootHash(root common.Hash) {}
func (t *dummyMptTracer) GetContract(addr types.Address) (*types.SmartContract, error) {
	code, ok := t.accountCodes[addr]
	if !ok {
		return nil, errors.New("no code for account found")
	}
	return &types.SmartContract{
		Address:  addr,
		CodeHash: code.Hash(),
	}, nil
}

func (t *dummyMptTracer) UpdateContracts(contracts map[types.Address]*execution.AccountState) error {
	panic("UpdateContracts")
}

func (t *dummyMptTracer) RootHash() common.Hash {
	return common.EmptyHash
}

func TestTracerMockClientTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TracerMockClientTestSuite))
}

func (s *TracerMockClientTestSuite) SetupSuite() {
	s.shardId = types.MainShardId
	// TODO(@makxenov): for some reason this works only for strings of length <= 18
	s.smartAccount = types.BytesToAddress([]byte("smart account"))
}

func (s *TracerMockClientTestSuite) SetupTest() {
	s.rpcClient = s.makeClient()
	s.accounts = map[types.Address]types.Code{}
	s.accounts[s.smartAccount] = []byte{}
	s.inMsgs = nil // remove transactions from previous test
}

func (s *TracerMockClientTestSuite) addContract(addr types.Address, code []byte) {
	s.T().Helper()
	s.Require().Equal(addr.ShardId(), s.shardId)
	s.accounts[addr] = code
}

func (s *TracerMockClientTestSuite) addCallContractTransaction(addr types.Address) {
	s.inMsgs = append(s.inMsgs, &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To:           addr,
			FeeCredit:    types.GasToValue(1000000),
			MaxFeePerGas: types.DefaultGasPrice,
		},
		From: s.smartAccount,
	})
}

func (s *TracerMockClientTestSuite) getInTrasactionsRoot() common.Hash {
	s.T().Helper()
	if len(s.inMsgs) == 0 {
		return common.EmptyHash
	}

	inTransactionTree := execution.NewTransactionTrie(mpt.NewInMemMPT())
	inKeys := make([]types.TransactionIndex, 0, len(s.inMsgs))
	inValues := make([]*types.Transaction, 0, len(s.inMsgs))
	for i, tx := range s.inMsgs {
		inKeys = append(inKeys, types.TransactionIndex(i))
		inValues = append(inValues, tx)
	}

	err := inTransactionTree.UpdateBatch(inKeys, inValues)
	s.Require().NoError(err)
	return inTransactionTree.RootHash()
}

func (s *TracerMockClientTestSuite) makeClient() client.Client {
	s.T().Helper()
	cl := &client.ClientMock{}
	cl.GetDebugContractFunc = func(
		_ context.Context, contractAddr types.Address, blockId any,
	) (*jsonrpc.DebugRPCContract, error) {
		s.T().Helper()

		code, exists := s.accounts[contractAddr]
		s.Require().True(exists)
		contract := types.SmartContract{
			Address:  contractAddr,
			Balance:  types.GasToValue(100000),
			CodeHash: code.Hash(),
			Seqno:    100,
			ExtSeqno: 100,
		}
		contractData, err := contract.MarshalSSZ()
		s.Require().NoError(err)

		// Build empty proof
		key := []byte{0x1}
		proof, err := mpt.BuildProof(mpt.NewInMemMPT().Reader, key, mpt.ReadMPTOperation)
		s.Require().NoError(err)
		encodedProof, err := proof.Encode()
		s.Require().NoError(err)

		return &jsonrpc.DebugRPCContract{
			Contract: contractData,
			Code:     []byte(code),
			Proof:    encodedProof,
			Storage:  make(map[common.Hash]types.Uint256),
		}, nil
	}

	cl.GetDebugBlockFunc = func(
		_ context.Context, shardId types.ShardId, blockId any, fullTx bool,
	) (*jsonrpc.DebugRPCBlock, error) {
		s.T().Helper()

		blockNrOrHash, err := transport.AsBlockReference(blockId)
		if err != nil {
			return nil, err
		}

		if blockNrOrHash.BlockHash != nil {
			blkNum := transport.BlockNumber(1)
			blockNrOrHash.BlockNumber = &blkNum
		}

		block := &types.Block{
			BlockData: types.BlockData{
				Id:                 blockNrOrHash.BlockNumber.BlockNumber(),
				BaseFee:            types.DefaultGasPrice,
				SmartContractsRoot: common.EmptyHash,
				InTransactionsRoot: s.getInTrasactionsRoot(),
			},
		}
		blockWithData := &types.BlockWithExtractedData{
			Block:          block,
			InTransactions: s.inMsgs,
			Config:         make(map[string]hexutil.Bytes),
		}
		rawBlock, err := blockWithData.EncodeSSZ()
		s.Require().NoError(err)
		return jsonrpc.EncodeRawBlockWithExtractedData(rawBlock)
	}

	cl.GetBlockFunc = func(
		_ context.Context, shardId types.ShardId, blockId any, fullTx bool,
	) (*jsonrpc.RPCBlock, error) {
		magicBytes := []byte{1, 2, 3, 4, 5}
		return &jsonrpc.RPCBlock{
			MainShardHash: common.BytesToHash(magicBytes),
		}, nil
	}

	return cl
}

func (s *TracerMockClientTestSuite) simpleContractCallTrace(code []byte) *ExecutionTraces {
	s.T().Helper()
	addr := types.ShardAndHexToAddress(s.shardId, "0x1234")
	s.addContract(addr, code)
	s.addCallContractTransaction(addr)

	prevBlock := types.BlockWithExtractedData{Block: &types.Block{BlockData: types.BlockData{Id: 1}}}
	db, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	chainConfig := jsonrpc.ChainConfig{}
	rwTx, err := db.CreateRwTx(context.Background())
	s.Require().NoError(err)
	configMap, err := chainConfig.ToMap()
	s.Require().NoError(err)
	configAccessor := config.NewConfigAccessorFromMap(configMap)

	es, err := execution.NewExecutionState(
		rwTx,
		s.shardId,
		execution.StateParams{
			Block:          prevBlock.Block,
			ConfigAccessor: configAccessor,
		},
	)
	s.Require().NoError(err)

	esTracer := NewEVMTracer(es)

	mptTracer, err := newDummyMptTracer(s.accounts, s.shardId, rwTx)
	s.Require().NoError(err)
	es.ContractTree = mptTracer

	es.EvmTracingHooks = esTracer.getTracingHooks()

	tx := types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:     types.TransactionFlagsFromKind(true, types.ExecutionTransactionKind),
			To:        types.ShardAndHexToAddress(s.shardId, "0x1234"),
			FeeCredit: types.NewValueFromUint64(100500100500),
		},
		From: s.smartAccount,
	}
	esTracer.initTransactionTraceContext(tx.Hash())
	es.AddInTransaction(&tx)

	caller := (vm.AccountRef)(tx.From)
	callData := tx.Data

	gas := tx.FeeCredit.ToGas(types.NewValueFromUint64(20))
	blockContext, err := execution.NewEVMBlockContext(es)
	s.Require().NoError(err)
	evm := vm.NewEVM(blockContext, es, tx.From, es.GasPrice)
	evm.Config.Tracer = es.EvmTracingHooks
	_, _, err = evm.Call(caller, tx.To, callData, gas.Uint64(), tx.Value.Int())
	s.Require().NoError(err)

	err = esTracer.saveTransactionTraces()
	s.Require().NoError(err)
	return esTracer.Traces
}

func (s *TracerMockClientTestSuite) TestSinglePush() {
	code := []byte{
		byte(vm.PUSH1), 4,
		byte(vm.STOP),
	}
	traceData := s.simpleContractCallTrace(code)

	// Check rw and copy operations
	s.Require().Len(traceData.StackOps, 1)
	s.Require().False(traceData.StackOps[0].IsRead)
	s.Require().Empty(traceData.MemoryOps)
	s.Require().Empty(traceData.StorageOps)
	s.Require().Empty(traceData.CopyEvents, 0)

	// Check bytecodes
	s.Require().Len(traceData.ContractsBytecode, 1)
	var actualCode []byte
	for _, code := range traceData.ContractsBytecode {
		actualCode = code
	}
	s.Require().Equal(code, actualCode)

	// Check zkEVM states
	s.Require().Len(traceData.ZKEVMStates, 2)
	s.Equal(vm.PUSH1, traceData.ZKEVMStates[0].OpCode)
	s.Equal(*types.NewUint256(4), traceData.ZKEVMStates[0].AdditionalInput)
	s.Equal(uint(0), traceData.ZKEVMStates[0].RwIdx)
	s.Equal(vm.STOP, traceData.ZKEVMStates[1].OpCode)
	s.Equal(uint(1), traceData.ZKEVMStates[1].RwIdx)
}

func (s *TracerMockClientTestSuite) TestMCOPY() {
	dataToCopy, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	s.Require().NoError(err)
	s.Require().Len(dataToCopy, 32)
	initialOffset := 32
	destOffset := 0
	initPush := append([]byte{byte(vm.PUSH32)}, dataToCopy...)
	code := slices.Concat(initPush, []byte{
		byte(vm.PUSH1), byte(initialOffset),
		byte(vm.MSTORE),

		byte(vm.PUSH1), byte(len(dataToCopy)),
		byte(vm.PUSH1), byte(initialOffset),
		byte(vm.PUSH1), byte(destOffset),
		byte(vm.MCOPY),
		byte(vm.STOP),
	})
	traceData := s.simpleContractCallTrace(code)

	// Check copy event
	s.Require().Len(traceData.CopyEvents, 1)
	copyEvent := traceData.CopyEvents[0]
	s.Equal(dataToCopy, copyEvent.Data)
	s.Equal(CopyLocationMemory, copyEvent.From.Location)
	s.Equal(uint64(32), copyEvent.From.MemAddress)
	s.Equal(uint64(0), copyEvent.To.MemAddress)

	// Check stack ops
	s.Require().Len(traceData.StackOps, 5+2+3) // 5 pushes, 2 reads for mstore and 3 reads for mcopy

	// Check memory ops
	s.Require().Len(traceData.MemoryOps, 32*3)
	for i, op := range traceData.MemoryOps {
		switch {
		case i < 32:
			// Writing data from stack into memory
			s.False(op.IsRead)
			s.Equal(dataToCopy[i], op.Value)
			s.Equal(initialOffset+i, op.Idx)
		case i >= 32 && i < 32*2:
			// Read operations
			s.True(op.IsRead)
			readIdx := i - 32 // local index among read operations
			s.Equal(dataToCopy[readIdx], op.Value)
			s.Equal(initialOffset+readIdx, op.Idx)
		case i >= 32*2:
			// Write operations while copying
			s.False(op.IsRead)
			writeIdx := i - 32*2 // local index among write operations
			s.Equal(dataToCopy[writeIdx], op.Value)
			s.Equal(destOffset+writeIdx, op.Idx)
		}
	}
}

func (s *TracerMockClientTestSuite) TestStorageOps() {
	code := []byte{
		byte(vm.PUSH1), 46,
		byte(vm.PUSH1), 2,
		byte(vm.SSTORE),

		// Example 1
		byte(vm.PUSH1), 2,
		byte(vm.SLOAD),

		// Example 2
		byte(vm.PUSH1), 1,
		byte(vm.SLOAD),
		byte(vm.STOP),
	}

	traceData := s.simpleContractCallTrace(code)
	s.Require().Len(traceData.StorageOps, 3)
	// s.Require().Equal(traceData.StorageOps[0].PC, traceData.StorageOps[1].PC)
	s.Require().False(traceData.StorageOps[0].IsRead)
	s.Require().True(traceData.StorageOps[1].IsRead)
	s.Require().True(traceData.StorageOps[2].IsRead)
}

// Zero-sized copy ops are not expected to be processed by copy circuit
// Check if they are skipped
func (s *TracerMockClientTestSuite) TestEmptyMCOPY() {
	code := []byte{
		byte(vm.PUSH1), 0xFF,
		byte(vm.PUSH1), 12,
		byte(vm.MSTORE),

		byte(vm.PUSH1), 0, // zero size
		byte(vm.PUSH1), 12, // offset
		byte(vm.PUSH1), 0, // destination offset
		byte(vm.MCOPY),
		byte(vm.STOP),
	}
	traceData := s.simpleContractCallTrace(code)
	s.Require().Empty(traceData.CopyEvents)
}

// Ensure that POP does not produce any stack operation
func (s *TracerMockClientTestSuite) TestPOP() {
	code := []byte{
		byte(vm.PUSH1), 0xFF,
		byte(vm.POP),
		byte(vm.STOP),
	}
	traceData := s.simpleContractCallTrace(code)
	s.Require().Len(traceData.StackOps, 1)
	s.Require().False(traceData.StackOps[0].IsRead)
}

// Ensure that we got zero data from uninitialized memory on MLOAD
func (s *TracerMockClientTestSuite) TestLoadFromUninitialized() {
	code := []byte{
		byte(vm.PUSH1), 0xBA,
		byte(vm.MLOAD),
		byte(vm.STOP),
	}
	traceData := s.simpleContractCallTrace(code)
	s.Require().Len(traceData.MemoryOps, 32)
	for _, op := range traceData.MemoryOps {
		s.Equal(byte(0), op.Value)
	}
}

// Check that copyed code was extended with zeros in case of insufficient size for copy
func (s *TracerMockClientTestSuite) TestInsufficientCodeCopy() {
	code := []byte{
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0,
		byte(vm.CODECOPY),
	}
	traceData := s.simpleContractCallTrace(code)
	s.Require().Len(traceData.CopyEvents, 1)
	copyEvent := traceData.CopyEvents[0]
	s.Require().Equal(CopyLocationBytecode, copyEvent.From.Location)
	s.Require().Equal(append(code, make([]byte, 32-len(code))...), copyEvent.Data)
}

// Check sequence of stack operations on SWAP
func (s *TracerMockClientTestSuite) TestSwapOperations() {
	code := []byte{
		// Set state
		byte(vm.PUSH1), 2,
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 1,

		// Swap
		byte(vm.SWAP4),
		byte(vm.STOP),
	}
	traceData := s.simpleContractCallTrace(code)
	s.Require().Len(traceData.StackOps, 9) //  5 pushes plus 4 on the swap
	swapOps := traceData.StackOps[5:]
	// Following sequence expected: read from top, read from swap, write top to swap, write swap to top
	s.True(swapOps[0].IsRead)
	s.Equal(*types.NewUint256(1), swapOps[0].Value)
	s.True(swapOps[1].IsRead)
	s.Equal(*types.NewUint256(2), swapOps[1].Value)
	s.False(swapOps[2].IsRead)
	s.Equal(*types.NewUint256(1), swapOps[2].Value)
	s.False(swapOps[3].IsRead)
	s.Equal(*types.NewUint256(2), swapOps[3].Value)
}

// Check that copy event finalizer is called
func (s *TracerMockClientTestSuite) TestKeccakOpCodeTracing() {
	// Check two keccak opcodes, since they are finalized in different place:
	// we have finalizer call after the previous opcode and at the end of transaction
	dataToCopy, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	expectedHash := crypto.Keccak256(dataToCopy)

	s.Require().NoError(err)
	initPush := append([]byte{byte(vm.PUSH32)}, dataToCopy...)
	code := slices.Concat(initPush, []byte{
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),

		byte(vm.PUSH1), 32, // size
		byte(vm.PUSH1), 0, // offset
		byte(vm.KECCAK256),
		byte(vm.PUSH1), 32, // size
		byte(vm.PUSH1), 0, // offset
		byte(vm.KECCAK256),
		byte(vm.STOP),
	})
	traceData := s.simpleContractCallTrace(code)

	// check copy event tracer finalizer
	s.Require().Len(traceData.CopyEvents, 2)
	s.EqualValues(expectedHash, *traceData.CopyEvents[0].To.KeccakHash)
	s.EqualValues(expectedHash, *traceData.CopyEvents[1].To.KeccakHash)

	// check keccak tracer
	s.Require().Len(traceData.KeccakTraces, 2)
	s.Equal(dataToCopy, traceData.KeccakTraces[0].buf)
	s.EqualValues(expectedHash, traceData.KeccakTraces[0].hash)
	s.Equal(dataToCopy, traceData.KeccakTraces[1].buf)
	s.EqualValues(expectedHash, traceData.KeccakTraces[1].hash)
}
