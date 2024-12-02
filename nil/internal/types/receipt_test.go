package types

import (
	"encoding/hex"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/require"
)

func TestReceiptEncoding(t *testing.T) {
	t.Parallel()

	var buf []byte

	data, err := hex.DecodeString("11223344aabbccdd")
	require.NoError(t, err)

	h1 := common.HexToHash("55555555555555555555")
	h2 := common.HexToHash("77777777777777777777")
	topics := []common.Hash{h1, h2}
	log, err := NewLog(HexToAddress("0xbbbbbbbbb"), data, topics)
	require.NoError(t, err)

	receipt := &Receipt{Success: true, GasUsed: 123}
	receipt.Logs = append(receipt.Logs, log)

	h3 := common.HexToHash("eeeeeeeeeeeeeeeeeeee")
	h4 := common.HexToHash("cccccccccccccccccccc")
	data, err = hex.DecodeString("abcdef0123456789")
	require.NoError(t, err)
	topics = []common.Hash{h1, h2, h3, h4}

	log, err = NewLog(HexToAddress("0xaaaaaaaa"), data, topics)
	require.NoError(t, err)
	receipt.Logs = append(receipt.Logs, log)

	buf, err = receipt.MarshalSSZ()
	require.NoError(t, err)

	receiptDecoded := &Receipt{}
	require.NoError(t, receiptDecoded.UnmarshalSSZ(buf))

	require.Equal(t, receiptDecoded.Success, receipt.Success)
	require.Equal(t, receiptDecoded.GasUsed, receipt.GasUsed)
	require.Equal(t, receiptDecoded.Bloom, receipt.Bloom)
	require.Equal(t, len(receiptDecoded.Logs), len(receipt.Logs))
	for i := range receipt.Logs {
		log1 := receipt.Logs[i]
		log2 := receiptDecoded.Logs[i]
		require.Equal(t, log1.Address, log2.Address)
		require.Equal(t, log1.Data, log2.Data)
		require.Equal(t, len(log1.Topics), len(log2.Topics))
		for j := range log1.Topics {
			t1 := log1.Topics[j]
			t2 := log2.Topics[j]
			require.Equal(t, t1, t2)
		}
	}
}
