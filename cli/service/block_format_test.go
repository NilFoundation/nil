package service

import (
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

func makeCurrencies(balances ...uint64) []types.CurrencyBalance {
	currencies := make([]types.CurrencyBalance, len(balances))
	for i, balance := range balances {
		currencies[i] = types.CurrencyBalance{
			Currency: types.CurrencyId(common.IntToHash(int(balance))),
			Balance:  types.NewValueFromUint64(balance * 100),
		}
	}
	return currencies
}

func TestDebugBlockToText(t *testing.T) {
	t.Parallel()

	message1 := &types.Message{
		Flags:     types.MessageFlagsFromKind(true, types.ExecutionMessageKind),
		ChainId:   1,
		Seqno:     0,
		From:      types.BytesToAddress(hexutil.FromHex("0x01")),
		To:        types.BytesToAddress(hexutil.FromHex("0x02")),
		RefundTo:  types.BytesToAddress(hexutil.FromHex("0x03")),
		BounceTo:  types.BytesToAddress(hexutil.FromHex("0x04")),
		Value:     types.NewValueFromUint64(300),
		Currency:  makeCurrencies(0x666, 0x777),
		Data:      hexutil.FromHex("0xDEADC0DE"),
		Signature: nil,
	}

	message2 := &types.Message{
		Flags:     types.MessageFlagsFromKind(false, types.DeployMessageKind),
		ChainId:   1,
		Seqno:     0,
		From:      types.BytesToAddress(hexutil.FromHex("0x0100")),
		To:        types.BytesToAddress(hexutil.FromHex("0x0200")),
		RefundTo:  types.BytesToAddress(hexutil.FromHex("0x0300")),
		BounceTo:  types.BytesToAddress(hexutil.FromHex("0x0400")),
		Value:     types.NewValueFromUint64(0),
		Currency:  nil,
		Data:      types.Code{},
		Signature: []byte("Signature"),
	}

	message3 := &types.Message{
		Flags:    types.MessageFlagsFromKind(true, types.ExecutionMessageKind),
		ChainId:  1,
		Seqno:    0,
		From:     types.BytesToAddress(hexutil.FromHex("0x0200")),
		To:       types.BytesToAddress(hexutil.FromHex("0x999")),
		RefundTo: types.BytesToAddress(hexutil.FromHex("0x0")),
		BounceTo: types.BytesToAddress(hexutil.FromHex("0x0")),
		Value:    types.NewValueFromUint64(1234),
		Currency: makeCurrencies(0x888),
		Data: hexutil.FromHex("0x" +
			"0000000000" +
			"1111111111" +
			"2222222222" +
			"3333333333" +
			"4444444444" +
			"5555555555" +
			"6666666666" +
			"7777777777" +
			"8888888888" +
			"9999999999" +
			"AAAAAAAAAA" +
			"BBBBBBBBBB" +
			"CCCCCCCCCC" +
			"DDDDDDDDDD" +
			"EEEEEEEEEE" +
			"FFFFFFFFFF"),
	}

	receipt1 := &types.Receipt{
		Success:         true,
		Status:          types.MessageStatusSuccess,
		GasUsed:         1000,
		Bloom:           types.Bloom{},
		Logs:            []*types.Log{},
		OutMsgIndex:     10,
		OutMsgNum:       2,
		MsgHash:         message1.Hash(),
		ContractAddress: message1.To,
	}

	receipt2 := &types.Receipt{
		Success:         false,
		Status:          types.MessageStatusExecutionReverted,
		GasUsed:         1500,
		Bloom:           types.Bloom{},
		Logs:            []*types.Log{},
		OutMsgIndex:     0,
		OutMsgNum:       0,
		MsgHash:         message2.Hash(),
		ContractAddress: message2.To,
	}

	block := &types.BlockWithExtractedData{
		Block: &types.Block{
			Id:                  types.BlockNumber(100500),
			PrevBlock:           common.HexToHash("0xDEADBEEF"),
			SmartContractsRoot:  common.HexToHash("0xDEADC0DE"),
			InMessagesRoot:      common.HexToHash("0xDEADCAFE"),
			OutMessagesRoot:     common.HexToHash("0xDEADF00D"),
			ReceiptsRoot:        common.HexToHash("0xD15EA5E"),
			ChildBlocksRootHash: common.HexToHash("0xDEADBABE"),
			MainChainHash:       common.HexToHash("0xB16B055"),
			LogsBloom:           types.Bloom{},
			Timestamp:           0x12345678,
		},
		InMessages:  []*types.Message{message1, message2},
		OutMessages: []*types.Message{message3},
		Receipts:    []*types.Receipt{receipt1, receipt2},
		Errors: map[common.Hash]string{
			message2.Hash():           "Error message",
			common.HexToHash("0xBAD"): "Another error message",
		},
	}

	s := NewService(nil, nil)
	t.Run("FilledBlock", func(t *testing.T) {
		t.Parallel()

		text, err := s.debugBlockToText(types.ShardId(13), block, false, false)
		require.NoError(t, err)

		expectedText := `Block #100500 [0x144c8b668b3bf0b6a780ae508618010cf4e39f6bcdedc3f5e37d5084b6243087] @ 13 shard
  PrevBlock: 0x00000000000000000000000000000000000000000000000000000000deadbeef
  ChildBlocksRootHash: 0x00000000000000000000000000000000000000000000000000000000deadbabe
  MainChainHash: 0x000000000000000000000000000000000000000000000000000000000b16b055
▼ InMessages [0x00000000000000000000000000000000000000000000000000000000deadcafe]:
  # 0 [0x015560664cb3e4c8f210c516fe98667d35b5fc8eb7b403e9259c91412411b308] | 0x0000000000000000000000000000000000000001 => 0x0000000000000000000000000000000000000002
    Status: Success
    GasUsed: 1000
    Flags: Internal
    RefundTo: 0x0000000000000000000000000000000000000003
    BounceTo: 0x0000000000000000000000000000000000000004
    Value: 300
    ChainId: 1
    Seqno: 0
  ▼ Currency:
      0x0000000000000000000000000000000000000000000000000000000000000666: 163800
      0x0000000000000000000000000000000000000000000000000000000000000777: 191100
    Data: 0xdeadc0de
  # 1 [0x1f3e2418afa43b5289c87778f1843f588a29e73ce65b718e285cd976c115cd15] | 0x0000000000000000000000000000000000000100 => 0x0000000000000000000000000000000000000200
    Status: ExecutionReverted
    GasUsed: 1500
    Error: Error message
    Flags: External, Deploy
    RefundTo: 0x0000000000000000000000000000000000000300
    BounceTo: 0x0000000000000000000000000000000000000400
    Value: 0
    ChainId: 1
    Seqno: 0
    Data: <empty>
    Signature: [83 105 103 110 97 116 117 114 101]
▼ OutMessages [0x00000000000000000000000000000000000000000000000000000000deadf00d]:
  # 0 [0x1f61e819c9ecb6ad065e149990a381be78038bf03ec9e39ef9693d9a0201de10] | 0x0000000000000000000000000000000000000200 => 0x0000000000000000000000000000000000000999
    Flags: Internal
    RefundTo: 0x0000000000000000000000000000000000000000
    BounceTo: 0x0000000000000000000000000000000000000000
    Value: 1234
    ChainId: 1
    Seqno: 0
  ▼ Currency:
      0x0000000000000000000000000000000000000000000000000000000000000888: 218400
    Data: 0x00000000001111111111222222222233333333334444444444555555555566666666667777777777888888888899999999... (run with --full to expand)
▼ Receipts [0x000000000000000000000000000000000000000000000000000000000d15ea5e]:
  [0x015560664cb3e4c8f210c516fe98667d35b5fc8eb7b403e9259c91412411b308]
     Status: Success
     GasUsed: 1000
  [0x1f3e2418afa43b5289c87778f1843f588a29e73ce65b718e285cd976c115cd15]
     Status: ExecutionReverted
     GasUsed: 1500
▼ Errors:
    0x0000000000000000000000000000000000000000000000000000000000000bad: Another error message
    0x1f3e2418afa43b5289c87778f1843f588a29e73ce65b718e285cd976c115cd15: Error message`

		require.Equal(t, expectedText, string(text))
	})

	t.Run("EmptyBlock", func(t *testing.T) {
		t.Parallel()
		emptyBlock := *block

		emptyBlock.InMessages = nil
		emptyBlock.OutMessages = nil
		emptyBlock.Receipts = nil
		emptyBlock.Errors = nil

		text, err := s.debugBlockToText(types.ShardId(13), &emptyBlock, true, false)
		require.NoError(t, err)

		fmt.Println(string(text))
	})
}
