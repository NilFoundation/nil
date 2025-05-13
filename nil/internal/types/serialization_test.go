package types

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/require"
)

func TestSerializeBlock(t *testing.T) {
	t.Parallel()

	block := Block{
		BlockData: BlockData{
			Id:                 1,
			PrevBlock:          common.Hash{0x01},
			SmartContractsRoot: common.Hash{0x02},
		},
	}

	encoded, err := block.MarshalNil()
	require.NoError(t, err)

	block2 := Block{}
	err = block2.UnmarshalNil(encoded)
	require.NoError(t, err)

	require.Equal(t, block2.Id, block.Id)
	require.Equal(t, block2.PrevBlock, block.PrevBlock)
	require.Equal(t, block2.SmartContractsRoot, block.SmartContractsRoot)

	h, err := common.Keccak(&block2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("24ff2a9b05d533266c08cc1906bffcd746ecd1a427255de2319f9a9a45fa1a75")
	require.NoError(t, err)

	require.Equal(t, common.BytesToHash(h2), common.BytesToHash(h[:]))
}

func TestSerializeTransaction(t *testing.T) {
	t.Parallel()

	transaction := Transaction{
		TransactionDigest: TransactionDigest{
			To:    Address{},
			Data:  Code{0x00000001},
			Seqno: 567,
		},
		From:  Address{},
		Value: NewValueFromUint64(1234),
	}

	encoded, err := transaction.MarshalNil()
	require.NoError(t, err)

	transaction2 := Transaction{}
	err = transaction2.UnmarshalNil(encoded)
	require.NoError(t, err)

	require.Equal(t, transaction2.From, transaction.From)
	require.Equal(t, transaction2.To, transaction.To)
	require.Equal(t, transaction2.Value, transaction.Value)
	require.Equal(t, transaction2.Data, transaction.Data)
	require.Equal(t, transaction2.Seqno, transaction.Seqno)
	require.True(t, bytes.Equal(transaction2.Signature, transaction.Signature))

	h, err := common.Keccak(&transaction2)
	require.NoError(t, err)

	h2 := common.HexToHash("5971272eefd23b6e2dcb47985ce10925171f5d33ef0afdf1601c4474a61d7615")
	require.Equal(t, h2, h)
}

func TestSerializeSmc(t *testing.T) {
	t.Parallel()

	smc := SmartContract{
		Address:     HexToAddress("1d9bc16f1a559"),
		Balance:     NewValueFromUint64(1234),
		StorageRoot: common.Hash{0x01},
		CodeHash:    common.Hash{0x02},
		Seqno:       567,
	}

	encoded, err := smc.MarshalNil()
	require.NoError(t, err)

	smc2 := SmartContract{}
	err = smc2.UnmarshalNil(encoded)
	require.NoError(t, err)

	require.Equal(t, smc.Address, smc2.Address)
	require.Equal(t, smc.Balance, smc2.Balance)
	require.Equal(t, smc.StorageRoot, smc2.StorageRoot)
	require.Equal(t, smc.CodeHash, smc2.CodeHash)
	require.Equal(t, smc.Seqno, smc2.Seqno)

	h, err := common.Keccak(&smc2)
	require.NoError(t, err)

	h2 := common.HexToHash("0x3b5a9082bf21cd09f9c9eecf60664721bb01b8f514519c9ef3602e3f327bf527")
	require.Equal(t, h2, common.BytesToHash(h[:]))
}
