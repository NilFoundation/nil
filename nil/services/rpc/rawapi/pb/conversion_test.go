package pb

import (
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestContract_PackUnpack(t *testing.T) {
	t.Parallel()

	seqno := types.Seqno(123)
	value := types.NewValueFromUint64(321)
	stateDiff := map[common.Hash]common.Hash{
		common.HexToHash("0xabcd"): common.HexToHash("0xabcd"),
	}
	args := rpctypes.Contract{
		Seqno:     &seqno,
		ExtSeqno:  nil,
		Code:      &hexutil.Bytes{0x1},
		Balance:   &value,
		State:     nil,
		StateDiff: &stateDiff,
	}

	callArgs := new(Contract).PackProtoMessage(args)
	require.NotNil(t, callArgs)

	data, err := proto.Marshal(callArgs)
	require.NoError(t, err)

	var unpacked Contract
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	unpackedArgs := unpacked.UnpackProtoMessage()
	assert.Equal(t, args, unpackedArgs)
}

func getCallArgs() rpctypes.CallArgs {
	return rpctypes.CallArgs{
		Flags:       types.NewTransactionFlags(types.TransactionFlagInternal),
		From:        nil,
		To:          types.GenerateRandomAddress(123),
		Fee:         types.NewFeePackFromGas(321),
		Value:       types.NewValueFromUint64(1111),
		Seqno:       9999,
		Data:        &hexutil.Bytes{0x1, 0x2, 0x3},
		Transaction: nil,
		ChainId:     1,
	}
}

func TestCallArgs_PackUnpack(t *testing.T) {
	t.Parallel()

	args := getCallArgs()
	callArgs := new(CallArgs).PackProtoMessage(args)
	require.NotNil(t, callArgs)

	data, err := proto.Marshal(callArgs)
	require.NoError(t, err)

	var unpacked CallArgs
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	unpackedArgs := unpacked.UnpackProtoMessage()
	assert.Equal(t, args, unpackedArgs)
}

func getStateOverrides() *rpctypes.StateOverrides {
	seqno := types.Seqno(123)
	value := types.NewValueFromUint64(321)
	stateDiff := map[common.Hash]common.Hash{
		common.HexToHash("0xabcd"): common.HexToHash("0xabcd"),
	}
	contract := rpctypes.Contract{
		Seqno:     &seqno,
		ExtSeqno:  nil,
		Code:      &hexutil.Bytes{0x1},
		Balance:   &value,
		State:     nil,
		StateDiff: &stateDiff,
	}

	return &rpctypes.StateOverrides{
		types.GenerateRandomAddress(1): contract,
	}
}

func TestStateOverrides_PackUnpack(t *testing.T) {
	t.Parallel()

	args := getStateOverrides()
	callArgs := new(StateOverrides).PackProtoMessage(args)
	require.NotNil(t, callArgs)

	data, err := proto.Marshal(callArgs)
	require.NoError(t, err)

	var unpacked StateOverrides
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	unpackedArgs := unpacked.UnpackProtoMessage()
	assert.Equal(t, args, unpackedArgs)
}

func getBlockRef() rawapitypes.BlockReferenceOrHashWithChildren {
	return rawapitypes.BlockReferenceAsBlockReferenceOrHashWithChildren(
		rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
}

func getBlockHashWithChildren() rawapitypes.BlockReferenceOrHashWithChildren {
	return rawapitypes.BlockHashWithChildrenAsBlockReferenceOrHashWithChildren(
		common.HexToHash("0xabcd"),
		[]common.Hash{
			common.HexToHash("0x1234"),
			common.HexToHash("0x5678"),
		})
}

func TestCallRequest_PackUnpack(t *testing.T) {
	t.Parallel()

	for i, ref := range []rawapitypes.BlockReferenceOrHashWithChildren{getBlockRef(), getBlockHashWithChildren()} {
		blockRef := ref
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			callArgs := getCallArgs()
			overrides := getStateOverrides()

			callReq := &CallRequest{}
			require.NoError(t, callReq.PackProtoMessage(callArgs, blockRef, overrides))

			data, err := proto.Marshal(callReq)
			require.NoError(t, err)

			var unpacked CallRequest
			require.NoError(t, proto.Unmarshal(data, &unpacked))

			unpackedCallArgs, unpackedBlockRef, unpackedOverrides, err := unpacked.UnpackProtoMessage()
			require.NoError(t, err)
			assert.Equal(t, callArgs, unpackedCallArgs)
			assert.Equal(t, blockRef, unpackedBlockRef)
			assert.Equal(t, overrides, unpackedOverrides)
		})
	}
}

func TestOutTransaction_PackUnpack(t *testing.T) {
	t.Parallel()

	value := types.NewValueFromUint64(321)
	nestedTransaction := &rpctypes.OutTransaction{
		TransactionSSZ:  hexutil.Bytes{0x11},
		ForwardKind:     types.ForwardKindRemaining,
		Data:            hexutil.Bytes{0x22},
		CoinsUsed:       value,
		OutTransactions: nil,
		BaseFee:         types.NewValueFromUint64(1),
		Error:           "test message",
	}

	args := &rpctypes.OutTransaction{
		TransactionSSZ:  hexutil.Bytes{0x1},
		ForwardKind:     types.ForwardKindNone,
		Data:            hexutil.Bytes{0x2},
		CoinsUsed:       value,
		OutTransactions: []*rpctypes.OutTransaction{nestedTransaction},
		BaseFee:         types.NewValueFromUint64(2),
	}

	callArgs := new(OutTransaction).PackProtoMessage(args)
	require.NotNil(t, callArgs)

	data, err := proto.Marshal(callArgs)
	require.NoError(t, err)

	var unpacked OutTransaction
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	unpackedArgs := unpacked.UnpackProtoMessage()
	assert.Equal(t, args, unpackedArgs)
}

func TestCallResponse_PackUnpack(t *testing.T) {
	t.Parallel()

	gp := types.NewValueFromUint64(123)
	value := types.NewValueFromUint64(321)
	outTxn := &rpctypes.OutTransaction{
		TransactionSSZ:  hexutil.Bytes{0x1},
		Data:            hexutil.Bytes{0x2},
		CoinsUsed:       value,
		OutTransactions: nil,
		BaseFee:         gp,
	}

	args := &rpctypes.CallResWithGasPrice{
		Data:            hexutil.Bytes{0x1},
		CoinsUsed:       value,
		OutTransactions: []*rpctypes.OutTransaction{outTxn},
		BaseFee:         gp,
	}

	callResp := &CallResponse{}
	require.NoError(t, callResp.PackProtoMessage(args, nil))

	data, err := proto.Marshal(callResp)
	require.NoError(t, err)

	var unpacked CallResponse
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	unpackedArgs, err := unpacked.UnpackProtoMessage()
	require.NoError(t, err)
	assert.Equal(t, args, unpackedArgs)
}

func TestErrorMap_PackUnpack(t *testing.T) {
	t.Parallel()

	invalidUTF8 := []byte{0xC3, 0x28}
	key := common.BytesToHash(invalidUTF8)
	errorMap := map[common.Hash]string{
		key: "Error",
	}

	block := &RawFullBlock{Errors: packErrorMap(errorMap)}
	data, err := proto.Marshal(block)
	require.NoError(t, err)

	var unpacked RawFullBlock
	require.NoError(t, proto.Unmarshal(data, &unpacked))

	errorsUnpacked := unpackErrorMap(unpacked.Errors)
	assert.Equal(t, errorMap, errorsUnpacked)

	errorMap = map[common.Hash]string{
		key:              string(invalidUTF8),
		common.EmptyHash: "Error2",
	}

	errors := packErrorMap(errorMap)
	require.Len(t, errors, 2)
	val, ok := errors[key.String()]
	require.True(t, ok)
	assert.Equal(t, &Error{Message: "<invalid UTF-8 string>"}, val)
}
