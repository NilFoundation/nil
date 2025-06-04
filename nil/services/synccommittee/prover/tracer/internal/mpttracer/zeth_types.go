package mpttracer

import (
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// BlockchainData represents the complete JSON structure
type BlockchainData struct {
	UncleBlocks  []jsonrpc.RPCBlock   `json:"uncle_blocks"`
	Receipts     []jsonrpc.RPCReceipt `json:"receipts"`
	Proofs       [][]json.RawMessage  `json:"proofs"`
	TxCount      hexutil.Uint64       `json:"transaction_count"`
	Balance      hexutil.Big          `json:"balance"`
	Code         hexutil.Bytes        `json:"code"`
	Storage      []json.RawMessage    `json:"storage"`
	Preimages    []json.RawMessage    `json:"preimages"`
	NextAccounts []json.RawMessage    `json:"next_accounts"`
	NextSlots    []json.RawMessage    `json:"next_slots"`
}

// FileProviderCache represents the structure of JSON cache file zeth uses
type FileProviderCache struct {
	ClientVersion    string                  `json:"client_version"`
	FullBlocks       []GetBlockCache         `json:"full_blocks"`
	UncleBlocks      []any                   `json:"uncle_blocks"` // no uncle blocks in nil chain
	Proofs           []GetProofCache         `json:"proofs"`
	Receipts         []GetReceiptCache       `json:"receipts"`
	TransactionCount []TransactionCountCache `json:"transaction_count"`
	Balance          []BalanceCache          `json:"balance"`
	Code             []CodeCache             `json:"code"`
	Storage          []StorageCache          `json:"storage"`
	Preimages        []PreimageCache         `json:"preimages"`
	NextAccounts     []NextAccountsCache     `json:"next_accounts"`
	NextSlots        []NextSlotsCache        `json:"next_slots"`
}

type GetBlockCache struct {
	Args  BlockArgs
	Block jsonrpc.RPCBlock
}

func (p GetBlockCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Block})
}

type BlockArgs struct {
	BlockNo uint64 `json:"block_no"`
	ShardID uint64 `json:"shard_id"`
}

type GetProofCache struct {
	Args  GetProofArgs
	Proof jsonrpc.EthProof
}

func (p GetProofCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Proof})
}

type GetProofArgs struct {
	BlockNo uint64        `json:"block_no"`
	Address types.Address `json:"address"`
	Indices []common.Hash `json:"indices"`
}

type GetReceiptCache struct {
	Args     BlockArgs
	Receipts []jsonrpc.RPCReceipt
}

func (p GetReceiptCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Receipts})
}

type TransactionCountArgs struct {
	BlockArgs
	Address types.Address `json:"address"`
}

type TransactionCountCache struct {
	Args  TransactionCountArgs
	Seqno types.Seqno
}

func (p TransactionCountCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Seqno})
}

type BalanceArgs struct {
	BlockArgs
	Address types.Address `json:"address"`
}

type BalanceCache struct {
	Args    BalanceArgs
	Balance types.Value
}

func (p BalanceCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Balance})
}

type CodeArgs struct {
	BlockArgs
	Address types.Address `json:"address"`
}

type CodeCache struct {
	Args CodeArgs
	Code types.Code
}

func (p CodeCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Code})
}

type StorageArgs struct {
	BlockArgs
	Address types.Address `json:"address"`
	Key     hexutil.U256  `json:"index"`
}

type StorageCache struct {
	Args    StorageArgs
	Storage hexutil.U256
}

func (p StorageCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Storage})
}

// Transaction represents a blockchain transaction
type Transaction struct {
	Flags                []string      `json:"flags"`
	Success              bool          `json:"success"`
	RequestID            hexutil.Uint  `json:"requestId"`
	Data                 hexutil.Bytes `json:"data"`
	BlockHash            common.Hash   `json:"blockHash"`
	BlockNumber          hexutil.Uint  `json:"blockNumber"`
	From                 types.Address `json:"from"`
	GasUsed              string        `json:"gasUsed"`
	FeeCredit            hexutil.Big   `json:"feeCredit"`
	MaxPriorityFeePerGas hexutil.Big   `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         hexutil.Big   `json:"maxFeePerGas"`
	Hash                 common.Hash   `json:"hash"`
	Seqno                hexutil.Big   `json:"seqno"`
	To                   types.Address `json:"to"`
	RefundTo             types.Address `json:"refundTo"`
	BounceTo             types.Address `json:"bounceTo"`
	Index                hexutil.Big   `json:"index"`
	Value                hexutil.Big   `json:"value"`
	Signature            hexutil.Bytes `json:"signature"`
}

// AccountProofRequest represents a request for an account proof
type AccountProofRequest struct {
	BlockNo uint64        `json:"block_no"`
	Address types.Address `json:"address"`
	Indices []int         `json:"indices"`
}

// AccountProofResult contains the result of an account proof
type AccountProofResult struct {
	Address      types.Address   `json:"address"`
	Balance      hexutil.Big     `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        hexutil.Big     `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	AccountProof []hexutil.Bytes `json:"accountProof"`
	StorageProof []StorageProof  `json:"storageProof"`
}

// StorageProof represents a storage proof
type StorageProof struct {
	Key   common.Hash     `json:"key"`
	Value hexutil.Big     `json:"value"`
	Proof []hexutil.Bytes `json:"proof"`
}

type PreimageArgs struct {
	Digest hexutil.U256 `json:"digest"`
}

type PreimageCache struct {
	Args     PreimageArgs
	Preimage hexutil.U256
}

func (p PreimageCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.Preimage})
}

// NextAccountsCache
type NextAccountsArgs struct {
	BlockNumber uint64       `json:"block_no"`
	Start       hexutil.U256 `json:"start"`
	MaxResults  uint64       `json:"max_results"`
	NoCode      bool         `json:"no_code"`
	NoStorage   bool         `json:"no_storage"`
	Incompletes bool         `json:"incompletes"`
}

type NextAccountsCache struct {
	Args         NextAccountsArgs
	NextAccounts AccountRangeResponse
}

func (p NextAccountsCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.NextAccounts})
}

type AccountRangeResponseEntry struct {
	Balance  hexutil.Big   `json:"balance"`
	Nonce    hexutil.Big   `json:"nonce"`
	Root     hexutil.U256  `json:"root"`
	CodeHash hexutil.U256  `json:"codeHash"`
	Address  types.Address `json:"address"`
	Key      hexutil.U256  `json:"key"`
}

// AccountRangeQueryResponse matches the main Rust response structure.
type AccountRangeResponse struct {
	Root     hexutil.U256                                `json:"root"`
	Accounts map[types.Address]AccountRangeResponseEntry `json:"accounts"`
	Next     *hexutil.U256                               `json:"next,omitempty"`
}

// StorageRangeArgs corresponds to the Rust struct with direct field mappings.
type StorageRangeArgs struct {
	BlockNo    uint64        `json:"block_no"`
	TxIndex    uint64        `json:"tx_index"`
	Address    types.Address `json:"address"`
	Start      hexutil.U256  `json:"start"`
	MaxResults uint64        `json:"max_results"`
}

type NextSlotsCache struct {
	Args      StorageRangeArgs
	NextSlots StorageRangeResponse
}

func (p NextSlotsCache) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{p.Args, p.NextSlots})
}

// StorageRangeResponseEntry represents a single storage slot entry.
type StorageRangeResponseEntry struct {
	Key   hexutil.Big `json:"key"`
	Value hexutil.Big `json:"value"`
}

// StorageRangeResponse corresponds to the Rust response with optional next_key.
type StorageRangeResponse struct {
	Storage map[hexutil.U256]StorageRangeResponseEntry `json:"storage"`
	NextKey *hexutil.U256                              `json:"nextKey,omitempty"`
}

// BlockAddressRequest represents a request with block number and address
// type BlockAddressRequest struct {
// 	BlockNo int           `json:"block_no"`
// 	Address types.Address `json:"address"`
// }

// // UnmarshalBlockchainData properly unmarshals the blockchain data
// func UnmarshalBlockchainData(data []byte) (*BlockchainData, error) {
// 	var result BlockchainData
// 	if err := json.Unmarshal(data, &result); err != nil {
// 		return nil, err
// 	}
// 	return &result, nil
// }

// // Custom unmarshal functions for different array items

// // UnmarshalFullBlock unmarshals a full block item
// func UnmarshalFullBlock(data []json.RawMessage) (*BlockReference, *Block, error) {
// 	var blockRef BlockReference
// 	var block Block

// 	if err := json.Unmarshal(data[0], &blockRef); err != nil {
// 		return nil, nil, err
// 	}

// 	if err := json.Unmarshal(data[1], &block); err != nil {
// 		return nil, nil, err
// 	}

// 	return &blockRef, &block, nil
// }

// // UnmarshalProof unmarshals a proof item
// func UnmarshalProof(data []json.RawMessage) (*AccountProofRequest, *AccountProofResult, error) {
// 	var request AccountProofRequest
// 	var result AccountProofResult

// 	if err := json.Unmarshal(data[0], &request); err != nil {
// 		return nil, nil, err
// 	}

// 	if err := json.Unmarshal(data[1], &result); err != nil {
// 		return nil, nil, err
// 	}

// 	return &request, &result, nil
// }

// // UnmarshalTxCount unmarshals a transaction count item
// func UnmarshalTxCount(data []json.RawMessage) (*BlockAddressRequest, *hexutil.Big, error) {
// 	var request BlockAddressRequest
// 	var count hexutil.Big

// 	if err := json.Unmarshal(data[0], &request); err != nil {
// 		return nil, nil, err
// 	}

// 	if err := json.Unmarshal(data[1], &count); err != nil {
// 		return nil, nil, err
// 	}

// 	return &request, &count, nil
// }

// // UnmarshalBalance unmarshals a balance item
// func UnmarshalBalance(data []json.RawMessage) (*BlockAddressRequest, *hexutil.Big, error) {
// 	var request BlockAddressRequest
// 	var balance hexutil.Big

// 	if err := json.Unmarshal(data[0], &request); err != nil {
// 		return nil, nil, err
// 	}

// 	if err := json.Unmarshal(data[1], &balance); err != nil {
// 		return nil, nil, err
// 	}

// 	return &request, &balance, nil
// }

// // UnmarshalCode unmarshals a code item
// func UnmarshalCode(data []json.RawMessage) (*BlockAddressRequest, *hexutil.Bytes, error) {
// 	var request BlockAddressRequest
// 	var code hexutil.Bytes

// 	if err := json.Unmarshal(data[0], &request); err != nil {
// 		return nil, nil, err
// 	}

// 	if err := json.Unmarshal(data[1], &code); err != nil {
// 		return nil, nil, err
// 	}

// 	return &request, &code, nil
// }

func (c *FileProviderCache) Append(other *FileProviderCache) {
	fmt.Printf("c = %s\n", c)
	c.FullBlocks = append(c.FullBlocks, other.FullBlocks...)
	c.UncleBlocks = append(c.UncleBlocks, other.UncleBlocks...)
	c.Proofs = append(c.Proofs, other.Proofs...)
	c.Receipts = append(c.Receipts, other.Receipts...)
}
