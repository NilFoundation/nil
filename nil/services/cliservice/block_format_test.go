package cliservice

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func makeTokens(balances ...uint64) []types.TokenBalance {
	tokens := make([]types.TokenBalance, len(balances))
	for i, balance := range balances {
		id := types.HexToAddress(fmt.Sprintf("0x%064x", balance))
		tokens[i] = types.TokenBalance{
			Token:   types.TokenId(id),
			Balance: types.NewValueFromUint64(balance * 100),
		}
	}
	return tokens
}

func TestDebugBlockToText(t *testing.T) {
	t.Parallel()

	transaction1 := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.TransactionFlagsFromKind(true, types.ExecutionTransactionKind),
			ChainId: 1,
			Seqno:   0,
			To:      types.BytesToAddress(ethcommon.FromHex("0x02")),
			Data:    ethcommon.FromHex("0xDEADC0DE"),
		},
		From:      types.BytesToAddress(ethcommon.FromHex("0x01")),
		RefundTo:  types.BytesToAddress(ethcommon.FromHex("0x03")),
		BounceTo:  types.BytesToAddress(ethcommon.FromHex("0x04")),
		Value:     types.NewValueFromUint64(300),
		Token:     makeTokens(0x666, 0x777),
		Signature: nil,
	}
	// set dst shard-id to 1
	binary.BigEndian.PutUint16(transaction1.To[:], 1)

	transaction2 := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.TransactionFlagsFromKind(false, types.DeployTransactionKind),
			ChainId: 1,
			Seqno:   0,
			To:      types.BytesToAddress(ethcommon.FromHex("0x0200")),
		},
		From:      types.BytesToAddress(ethcommon.FromHex("0x0100")),
		RefundTo:  types.BytesToAddress(ethcommon.FromHex("0x0300")),
		BounceTo:  types.BytesToAddress(ethcommon.FromHex("0x0400")),
		Value:     types.Value0,
		Token:     nil,
		Signature: []byte("Signature"),
	}

	transaction3 := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.TransactionFlagsFromKind(true, types.ExecutionTransactionKind),
			ChainId: 1,
			Seqno:   0,
			To:      types.BytesToAddress(ethcommon.FromHex("0x999")),
			Data: ethcommon.FromHex("0x" +
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
		},
		From:     types.BytesToAddress(ethcommon.FromHex("0x0200")),
		RefundTo: types.BytesToAddress(ethcommon.FromHex("0x0")),
		BounceTo: types.BytesToAddress(ethcommon.FromHex("0x0")),
		Value:    types.NewValueFromUint64(1234),
		Token:    makeTokens(0x888),
	}

	receipt1 := &types.Receipt{
		Success:         true,
		Status:          types.ErrorSuccess,
		GasUsed:         1000,
		Logs:            []*types.Log{},
		OutTxnIndex:     10,
		OutTxnNum:       2,
		TxnHash:         transaction1.Hash(),
		ContractAddress: transaction1.To,
	}

	receipt2 := &types.Receipt{
		Success:         false,
		Status:          types.ErrorExecutionReverted,
		GasUsed:         1500,
		Logs:            []*types.Log{},
		OutTxnIndex:     0,
		OutTxnNum:       0,
		TxnHash:         transaction2.Hash(),
		ContractAddress: transaction2.To,
	}

	block := &types.BlockWithExtractedData{
		Block: &types.Block{
			BlockData: types.BlockData{
				Id:                  types.BlockNumber(100500),
				PrevBlock:           common.HexToHash("0xDEADBEEF"),
				SmartContractsRoot:  common.HexToHash("0xDEADC0DE"),
				InTransactionsRoot:  common.HexToHash("0xDEADCAFE"),
				OutTransactionsRoot: common.HexToHash("0xDEADF00D"),
				ReceiptsRoot:        common.HexToHash("0xD15EA5E"),
				ChildBlocksRootHash: common.HexToHash("0xDEADBABE"),
				MainShardHash:       common.HexToHash("0xB16B055"),
				GasUsed:             1234,
			},
			LogsBloom: types.Bloom{},
		},
		ChildBlocks: []common.Hash{
			common.HexToHash("0x111"),
			common.HexToHash("0x222"),
		},
		InTransactions:  []*types.Transaction{transaction1, transaction2},
		OutTransactions: []*types.Transaction{transaction3},
		Receipts:        []*types.Receipt{receipt1, receipt2},
		Errors: map[common.Hash]string{
			transaction2.Hash():       "Error message",
			common.HexToHash("0xBAD"): "Another error message",
		},
	}

	s := NewService(t.Context(), nil, nil, nil)
	t.Run("FilledBlock", func(t *testing.T) {
		t.Parallel()

		text, err := s.debugBlockToText(types.ShardId(13), block, false, false)
		require.NoError(t, err)

		expectedText := `Block #100500 [0x000d1288cfedf4f9c748f80d9d0e3edb422c092a2a17725878e36aa9383c6bfa] @ 13 shard
  PrevBlock: 0x00000000000000000000000000000000000000000000000000000000deadbeef
  BaseFee: 0
  GasUsed: 1234
  ChildBlocksRootHash: 0x00000000000000000000000000000000000000000000000000000000deadbabe
  ChildBlocks:
    - 1: 0x0000000000000000000000000000000000000000000000000000000000000111
    - 2: 0x0000000000000000000000000000000000000000000000000000000000000222
  MainShardHash: 0x000000000000000000000000000000000000000000000000000000000b16b055
▼ InTransactions [0x00000000000000000000000000000000000000000000000000000000deadcafe]:
  # 0 [0x000157f10d1f445c85fc01eab48c12c475c54e42b830a62b4e52c87e36840720] | 0x0000000000000000000000000000000000000001 => 0x0001000000000000000000000000000000000002
    Status: Success
    GasUsed: 1000
    Flags: Internal
    RefundTo: 0x0000000000000000000000000000000000000003
    BounceTo: 0x0000000000000000000000000000000000000004
    Value: 300
    ChainId: 1
    Seqno: 0
  ▼ Token:
      0x0000000000000000000000000000000000000666: 163800
      0x0000000000000000000000000000000000000777: 191100
    Data: 0xdeadc0de
  # 1 [0x0000229a4072ac3329c9faf2257b15ac0052b6aff1561d33ee128c138cf7377e] | 0x0000000000000000000000000000000000000100 => 0x0000000000000000000000000000000000000200
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
    Signature: 0x5369676e6174757265
▼ OutTransactions [0x00000000000000000000000000000000000000000000000000000000deadf00d]:
  # 0 [0x0000646887dd2d7e58b5a939af0bea3367fa7a837e606f740af315c801be5268] | 0x0000000000000000000000000000000000000200 => 0x0000000000000000000000000000000000000999
    Flags: Internal
    RefundTo: 0x0000000000000000000000000000000000000000
    BounceTo: 0x0000000000000000000000000000000000000000
    Value: 1234
    ChainId: 1
    Seqno: 0
  ▼ Token:
      0x0000000000000000000000000000000000000888: 218400
    Data: 0x00000000001111111111222222222233333333334444444444555555555566666666667777777777888888888899999999... (run with --full to expand)
▼ Receipts [0x000000000000000000000000000000000000000000000000000000000d15ea5e]:
  [0x000157f10d1f445c85fc01eab48c12c475c54e42b830a62b4e52c87e36840720]
     Status: Success
     GasUsed: 1000
  [0x0000229a4072ac3329c9faf2257b15ac0052b6aff1561d33ee128c138cf7377e]
     Status: ExecutionReverted
     GasUsed: 1500
▼ Errors:
    0x0000000000000000000000000000000000000000000000000000000000000bad: Another error message
    0x0000229a4072ac3329c9faf2257b15ac0052b6aff1561d33ee128c138cf7377e: Error message`

		require.Equal(t, expectedText, string(text))
	})

	t.Run("EmptyBlock", func(t *testing.T) {
		t.Parallel()
		emptyBlock := *block

		emptyBlock.InTransactions = nil
		emptyBlock.OutTransactions = nil
		emptyBlock.Receipts = nil
		emptyBlock.Errors = nil

		_, err := s.debugBlockToText(types.ShardId(13), &emptyBlock, true, false)
		require.NoError(t, err)
	})
}
