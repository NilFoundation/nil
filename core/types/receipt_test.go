package types

import (
	"encoding/hex"
	"github.com/NilFoundation/nil/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReceiptEncoding(t *testing.T) {
	receipt := NewReceipt(true, 123)
	var buf []byte

	data, err := hex.DecodeString("11223344aabbccdd")
	require.NoError(t, err)

	h1 := common.HexToHash("55555555555555555555")
	h2 := common.HexToHash("77777777777777777777")
	topics := []*Topic{(*Topic)(&h1), (*Topic)(&h2)}
	log := NewLog(common.HexToAddress("0xbbbbbbbbb"), data, 0, topics)
	receipt.Logs.Append(log)

	h3 := common.HexToHash("eeeeeeeeeeeeeeeeeeee")
	h4 := common.HexToHash("cccccccccccccccccccc")
	data, err = hex.DecodeString("abcdef0123456789")
	require.NoError(t, err)
	topics = []*Topic{(*Topic)(&h1), (*Topic)(&h2), (*Topic)(&h3), (*Topic)(&h4)}
	log = NewLog(common.HexToAddress("0xaaaaaaaa"), data, 1, topics)
	receipt.Logs.Append(log)

	buf, err = receipt.EncodeSSZ(buf)
	require.NoError(t, err)

	receiptDecoded := NewReceipt(true, 123)
	require.NoError(t, receiptDecoded.DecodeSSZ(buf, 0))

	require.Equal(t, receiptDecoded.Success, receipt.Success)
	require.Equal(t, receiptDecoded.GasUsed, receipt.GasUsed)
	require.Equal(t, receiptDecoded.Bloom, receipt.Bloom)
	require.Equal(t, receiptDecoded.Logs.Len(), receipt.Logs.Len())
	for i := 0; i < receipt.Logs.Len(); i++ {
		log1 := receipt.Logs.Get(i)
		log2 := receiptDecoded.Logs.Get(i)
		require.Equal(t, log1.Address, log2.Address)
		require.Equal(t, log1.Data, log2.Data)
		require.Equal(t, log1.Topics.Len(), log2.Topics.Len())
		for j := 0; j < log1.Topics.Len(); j++ {
			t1 := log1.Topics.Get(j)
			t2 := log2.Topics.Get(j)
			require.Equal(t, t1, t2)
		}
	}
}
