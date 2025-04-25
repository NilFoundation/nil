package types_test

import (
	"bytes"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionDigestRLP(t *testing.T) {
	t.Parallel()

	td := types.TransactionDigest{
		Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
		FeePack: types.NewFeePack(),
		To:      types.Address{1, 2, 3},
		ChainId: types.ChainId(1),
		Seqno:   types.Seqno(42),
		Data:    []byte{0xde, 0xad, 0xbe, 0xef},
	}

	var buf bytes.Buffer
	require.NoError(t, td.EncodeRLP(&buf))

	var decoded types.TransactionDigest
	require.NoError(t, decoded.DecodeRLP(rlp.NewStream(&buf, 0)))

	assert.Equal(t, td, decoded)
}

func TestTransactionRLP(t *testing.T) {
	t.Parallel()

	tx := types.NewEmptyTransaction()
	tx.From = types.Address{0x1}
	tx.RefundTo = types.Address{0x2}
	tx.BounceTo = types.Address{0x3}
	tx.Value = types.NewZeroValue()
	tx.Token = []types.TokenBalance{{Token: types.TokenId{0x4}, Balance: types.NewZeroValue()}}
	tx.RequestId = 1234
	tx.RequestChain = []*types.AsyncRequestInfo{{Id: 1, Caller: types.Address{0x5}}}
	tx.Signature = []byte{0xaa, 0xbb, 0xcc}
	tx.Data = []byte{0xde, 0xad, 0xbe, 0xef}

	var buf bytes.Buffer
	require.NoError(t, tx.EncodeRLP(&buf))

	var decoded types.Transaction
	require.NoError(t, decoded.DecodeRLP(rlp.NewStream(&buf, 0)))

	assert.Equal(t, tx, &decoded)
}

func TestExternalTransactionRLP(t *testing.T) {
	t.Parallel()

	ext := types.ExternalTransaction{
		Kind:     types.ExecutionTransactionKind,
		FeePack:  types.NewFeePack(),
		To:       types.Address{0x42},
		ChainId:  types.ChainId(10),
		Seqno:    types.Seqno(77),
		Data:     []byte{1, 2, 3, 4},
		AuthData: []byte{0xaa, 0xbb, 0xcc},
	}

	var buf bytes.Buffer
	require.NoError(t, ext.EncodeRLP(&buf))

	var decoded types.ExternalTransaction
	require.NoError(t, decoded.DecodeRLP(rlp.NewStream(&buf, 0)))

	assert.Equal(t, ext, decoded)
}

func TestInternalTransactionPayloadRLP(t *testing.T) {
	t.Parallel()

	itp := types.InternalTransactionPayload{
		Kind:        types.RefundTransactionKind,
		Bounce:      true,
		FeeCredit:   types.NewZeroValue(),
		ForwardKind: types.ForwardKindValue,
		To:          types.Address{0xaa},
		RefundTo:    types.Address{0xbb},
		BounceTo:    types.Address{0xcc},
		Token:       []types.TokenBalance{{Token: types.TokenId{0xdd}, Balance: types.NewZeroValue()}},
		Value:       types.NewZeroValue(),
		Data:        []byte{0x11, 0x22, 0x33},
		RequestId:   555,
	}

	var buf bytes.Buffer
	require.NoError(t, itp.EncodeRLP(&buf))

	var decoded types.InternalTransactionPayload
	require.NoError(t, decoded.DecodeRLP(rlp.NewStream(&buf, 0)))

	assert.Equal(t, itp, decoded)
}
