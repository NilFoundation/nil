package jsonrpc

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type (
	Contract       = rpctypes.Contract
	CallArgs       = rpctypes.CallArgs
	StateOverrides = rpctypes.StateOverrides
)

// @component RPCInTransaction rpcInTransaction object "The transaction whose information is requested."
// @componentprop BlockHash blockHash string true "The hash of the block containing the transaction."
// @componentprop BlockNumber blockNumber integer true "The number of the block containing the transaction."
// @componentprop ChainId chainId integer true "The number of the chain containing the transaction."
// @componentprop From from string true "The address from where the transaction was sent."
// @componentprop FeeCredit feeCredit string true "The fee credit for the transaction."
// @componentprop MaxPriorityFeePerGas maxPriorityFeePerGas string true "Priority fee for the transaction."
// @componentprop MaxFeePerGas maxFeePerGas string true "Maximum fee per gas for the transaction."
// @componentprop GasUsed gasUsed string true "The amount of gas spent on the transaction."
// @componentprop Hash hash string true "The transaction hash."
// @componentprop Index index string true "The transaction index."
// @componentprop Seqno seqno string true "The sequence number of the transaction."
// @componentprop Signature signature string true "The transaction signature."
// @componentprop Success success boolean true "The flag that shows whether the transaction was successful."
// @componentprop Flags flags string true "The array of transaction flags."
// @componentprop To to string true "The address where the transaction was sent."
// @componentprop Value value string true "The transaction value."
// @componentprop Token value array true "Token values."
type RPCInTransaction struct {
	Flags                types.TransactionFlags `json:"flags"`
	Success              bool                   `json:"success"`
	RequestId            uint64                 `json:"requestId"`
	Data                 hexutil.Bytes          `json:"data"`
	BlockHash            common.Hash            `json:"blockHash"`
	BlockNumber          types.BlockNumber      `json:"blockNumber"`
	From                 types.Address          `json:"from"`
	GasUsed              types.Gas              `json:"gasUsed"`
	FeeCredit            types.Value            `json:"feeCredit,omitempty"`
	MaxPriorityFeePerGas types.Value            `json:"maxPriorityFeePerGas,omitempty"`
	MaxFeePerGas         types.Value            `json:"maxFeePerGas,omitempty"`
	Hash                 common.Hash            `json:"hash"`
	Seqno                hexutil.Uint64         `json:"seqno"`
	To                   types.Address          `json:"to"`
	RefundTo             types.Address          `json:"refundTo"`
	BounceTo             types.Address          `json:"bounceTo"`
	Index                hexutil.Uint64         `json:"index"`
	Value                types.Value            `json:"value"`
	Token                []types.TokenBalance   `json:"token,omitempty"`
	ChainID              types.ChainId          `json:"chainId,omitempty"`
	Signature            types.Signature        `json:"signature"`
}

// @component RPCBlock rpcBlock object "The block whose information was requested."
// @componentprop Hash hash string true "The hash of the block."
// @componentprop Transactions transactions array true "The transactions included in the block."
// @componentprop TransactionHashes string array true "The hashes of transactions included in the block."
// @componentprop Number number integer true "The block number."
// @componentprop L1Number number integer true "The L1 block number."
// @componentprop ParentHash parentHash string true "The hash of the parent block."
// @componentprop ReceiptsRoot receiptsRoot string true "The root of the block receipts."
// @componentprop ShardId shardId integer true "The ID of the shard where the block was generated."
type RPCBlock struct {
	Number              types.BlockNumber   `json:"number"`
	Hash                common.Hash         `json:"hash"`
	ParentHash          common.Hash         `json:"parentHash"`
	PatchLevel          uint32              `json:"patchLevel"`
	RollbackCounter     uint32              `json:"rollbackCounter"`
	InTransactionsRoot  common.Hash         `json:"inTransactionsRoot"`
	ReceiptsRoot        common.Hash         `json:"receiptsRoot"`
	ChildBlocksRootHash common.Hash         `json:"childBlocksRootHash"`
	ShardId             types.ShardId       `json:"shardId"`
	Transactions        []*RPCInTransaction `json:"transactions,omitempty"`
	TransactionHashes   []common.Hash       `json:"transactionHashes,omitempty"`
	ChildBlocks         []common.Hash       `json:"childBlocks"`
	MainShardHash       common.Hash         `json:"mainShardHash"`
	DbTimestamp         uint64              `json:"dbTimestamp"`
	BaseFee             types.Value         `json:"baseFee"`
	L1Number            uint64              `json:"l1Number"`
	LogsBloom           hexutil.Bytes       `json:"logsBloom,omitempty"`
	GasUsed             types.Gas           `json:"gasUsed,omitempty"`
}

type DebugRPCBlock struct {
	Content         hexutil.Bytes          `json:"content"`
	ChildBlocks     []common.Hash          `json:"childBlocks"`
	InTransactions  []hexutil.Bytes        `json:"inTransactions"`
	OutTransactions []hexutil.Bytes        `json:"outTransactions"`
	Receipts        []hexutil.Bytes        `json:"receipts"`
	Errors          map[common.Hash]string `json:"errors"`
	Config          *ChainConfig           `json:"config"`
}

type ChainConfig struct {
	Validators  *config.ParamValidators  `json:"validators"`
	GasPrices   *config.ParamGasPrice    `json:"gasPrices"`
	L1BlockInfo *config.ParamL1BlockInfo `json:"l1BlockInfo"`
}

func NewChainConfigFromMap(data map[string][]byte) (*ChainConfig, error) {
	if data == nil {
		return nil, nil
	}
	configAccessor := config.NewConfigAccessorFromMap(data)
	validators, err := config.GetParamValidators(configAccessor)
	if err != nil && !errors.Is(err, config.ErrParamNotFound) {
		return nil, err
	}
	gasPrices, err := config.GetParamGasPrice(configAccessor)
	if err != nil && !errors.Is(err, config.ErrParamNotFound) {
		return nil, err
	}
	l1BlockInfo, err := config.GetParamL1Block(configAccessor)
	if err != nil && !errors.Is(err, config.ErrParamNotFound) {
		return nil, err
	}
	return &ChainConfig{
		Validators:  validators,
		GasPrices:   gasPrices,
		L1BlockInfo: l1BlockInfo,
	}, nil
}

func (c *ChainConfig) ToMap() (map[string][]byte, error) {
	result := make(map[string][]byte)
	if c.Validators != nil {
		validators, err := c.Validators.MarshalSSZ()
		if err != nil {
			return nil, err
		}
		result[config.NameValidators] = validators
	}
	if c.GasPrices != nil {
		gasPrices, err := c.GasPrices.MarshalSSZ()
		if err != nil {
			return nil, err
		}
		result[config.NameGasPrice] = gasPrices
	}
	if c.L1BlockInfo != nil {
		l1BlockInfo, err := c.L1BlockInfo.MarshalSSZ()
		if err != nil {
			return nil, err
		}
		result[config.NameL1Block] = l1BlockInfo
	}
	return result, nil
}

func (b *DebugRPCBlock) Encode(block *types.RawBlockWithExtractedData) error {
	b.Content = block.Block
	b.ChildBlocks = block.ChildBlocks
	b.InTransactions = hexutil.FromBytesSlice(block.InTransactions)
	b.OutTransactions = hexutil.FromBytesSlice(block.OutTransactions)
	b.Receipts = hexutil.FromBytesSlice(block.Receipts)
	b.Errors = block.Errors

	if block.Config != nil {
		config, err := NewChainConfigFromMap(block.Config)
		if err != nil {
			return err
		}
		b.Config = config
	}

	return nil
}

func (b *DebugRPCBlock) Decode() (*types.RawBlockWithExtractedData, error) {
	decodedBlock := types.RawBlockWithExtractedData{
		Block:           b.Content,
		ChildBlocks:     b.ChildBlocks,
		InTransactions:  hexutil.ToBytesSlice(b.InTransactions),
		OutTransactions: hexutil.ToBytesSlice(b.OutTransactions),
		Receipts:        hexutil.ToBytesSlice(b.Receipts),
		Errors:          b.Errors,
	}
	if b.Config != nil {
		configMap, err := b.Config.ToMap()
		if err != nil {
			return nil, err
		}
		decodedBlock.Config = configMap
	}

	return &decodedBlock, nil
}

func (b *DebugRPCBlock) DecodeSSZ() (*types.BlockWithExtractedData, error) {
	block, err := b.Decode()
	if err != nil {
		return nil, err
	}
	return block.DecodeSSZ()
}

func EncodeRawBlockWithExtractedData(block *types.RawBlockWithExtractedData) (*DebugRPCBlock, error) {
	b := &DebugRPCBlock{}
	if err := b.Encode(block); err != nil {
		return nil, err
	}
	return b, nil
}

// @component RPCReceipt rpcReceipt object "The receipt whose structure is requested."
// @componentprop BlockHash blockHash string true "The hash of the block containing the transaction whose receipt is requested."
// @componentprop BlockNumber blockNumber integer true "The number of the block containing the transaction whose receipt is requested."
// @componentprop Bloom bloom string true "The receipt bloom filter."
// @componentprop ContractAddress contractAddress string true "The address of the contract that has originated the transaction whose receipt is requested."
// @componentprop GasUsed gasUsed string true "The amount of gas spent on the transaction whose receipt is requested."
// @componentprop GasPrice gasPrice string true "The gas price at the time of processing the transaction."
// @componentprop Logs logs array true "The logs attached to the receipt."
// @componentprop TransactionHash transactionHash string true "The hash of the transaction whose receipt is requested."
// @componentprop TransactionIndex transactionIndex integer true "The index of the transaction whose receipt is requested."
// @componentprop OutTxnIndex outTxnIndex integer true "The index of the outgoing transaction whose receipt is requested."
// @componentprop OutTxnNum outTxnNum integer true "The number of the outgoing transactions whose receipt is requested."
// @componentprop OutReceipts outputReceipts array true "Receipts of the outgoing transactions. Set to nil for transactions that have not yet been processed."
// @componentprop Success success boolean true "The flag that shows whether the transaction was successful."
// @componentprop Status status string false "Status shows concrete error of the executed transaction."
// @componentprop Temporary temporary boolean false "The flag that shows whether the transaction is temporary."
// @componentprop ErrorMessage errorTransaction string false "The error in case the transaction processing was unsuccessful."
// @componentprop Flags flags string true "The array of transaction flags."
type RPCReceipt struct {
	Flags           types.TransactionFlags `json:"flags"`
	Success         bool                   `json:"success"`
	Status          string                 `json:"status"`
	FailedPc        uint                   `json:"failedPc"`
	IncludedInMain  bool                   `json:"includedInMain"`
	GasUsed         types.Gas              `json:"gasUsed"`
	Forwarded       types.Value            `json:"forwarded"`
	GasPrice        types.Value            `json:"gasPrice"`
	Bloom           hexutil.Bytes          `json:"bloom,omitempty"`
	Logs            []*RPCLog              `json:"logs"`
	DebugLogs       []*RPCDebugLog         `json:"debugLogs"`
	OutTransactions []common.Hash          `json:"outTransactions"`
	OutReceipts     []*RPCReceipt          `json:"outputReceipts"`
	TxnHash         common.Hash            `json:"transactionHash"`
	ContractAddress types.Address          `json:"contractAddress"`
	BlockHash       common.Hash            `json:"blockHash"`
	BlockNumber     types.BlockNumber      `json:"blockNumber"`
	TxnIndex        types.TransactionIndex `json:"transactionIndex"`
	ShardId         types.ShardId          `json:"shardId"`
	Temporary       bool                   `json:"temporary,omitempty"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
}

type RPCLog struct {
	*types.Log
	BlockNumber types.BlockNumber `json:"blockNumber"`
}

type RPCDebugLog struct {
	Message string          `json:"message"`
	Data    []types.Uint256 `json:"data"`
}

func (re *RPCReceipt) IsComplete() bool {
	if re == nil || len(re.OutReceipts) != len(re.OutTransactions) {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.IsComplete() {
			return false
		}
	}
	return true
}

func (re *RPCReceipt) AllSuccess() bool {
	if !re.Success {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.AllSuccess() {
			return false
		}
	}
	return true
}

// IsCommitted returns true if the receipt is complete and its block is included in the main chain.
func (re *RPCReceipt) IsCommitted() bool {
	if re == nil || len(re.OutReceipts) != len(re.OutTransactions) {
		return false
	}
	if !re.IncludedInMain {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.IsCommitted() {
			return false
		}
	}
	return true
}

func NewRPCInTransaction(
	transaction *types.Transaction, receipt *types.Receipt, index types.TransactionIndex,
	blockHash common.Hash, blockId types.BlockNumber,
) (*RPCInTransaction, error) {
	hash := transaction.Hash()
	if receipt == nil || hash != receipt.TxnHash {
		return nil, errors.New("txn and receipt are not compatible")
	}

	result := &RPCInTransaction{
		Flags:                transaction.Flags,
		Success:              receipt.Success,
		RequestId:            transaction.RequestId,
		Data:                 hexutil.Bytes(transaction.Data),
		BlockHash:            blockHash,
		BlockNumber:          blockId,
		From:                 transaction.From,
		GasUsed:              receipt.GasUsed,
		FeeCredit:            transaction.FeeCredit,
		MaxPriorityFeePerGas: transaction.MaxPriorityFeePerGas,
		MaxFeePerGas:         transaction.MaxFeePerGas,
		Hash:                 hash,
		Seqno:                hexutil.Uint64(transaction.Seqno),
		To:                   transaction.To,
		RefundTo:             transaction.RefundTo,
		BounceTo:             transaction.BounceTo,
		Index:                hexutil.Uint64(index),
		Value:                transaction.Value,
		Token:                transaction.Token,
		ChainID:              transaction.ChainId,
		Signature:            transaction.Signature,
	}

	return result, nil
}

func NewRPCBlock(shardId types.ShardId, data *BlockWithEntities, fullTx bool) (*RPCBlock, error) {
	block := data.Block
	transactions := data.InTransactions
	receipts := data.Receipts
	childBlocks := data.ChildBlocks
	dbTimestamp := data.DbTimestamp

	if block == nil {
		return nil, nil
	}

	transactionsRes := make([]*RPCInTransaction, 0, len(transactions))
	transactionHashesRes := make([]common.Hash, 0, len(transactions))
	blockHash := block.Hash(shardId)
	blockId := block.Id
	if fullTx {
		for i, m := range transactions {
			txn, err := NewRPCInTransaction(m, receipts[i], types.TransactionIndex(i), blockHash, blockId)
			if err != nil {
				return nil, err
			}
			transactionsRes = append(transactionsRes, txn)
		}
	} else {
		for _, m := range transactions {
			transactionHashesRes = append(transactionHashesRes, m.Hash())
		}
	}

	// Set only non-empty bloom
	var bloom hexutil.Bytes
	for _, b := range block.LogsBloom {
		if b != 0 {
			bloom = block.LogsBloom.Bytes()
			break
		}
	}

	return &RPCBlock{
		Number:              blockId,
		Hash:                blockHash,
		ParentHash:          block.PrevBlock,
		PatchLevel:          block.PatchLevel,
		RollbackCounter:     block.RollbackCounter,
		InTransactionsRoot:  block.InTransactionsRoot,
		ReceiptsRoot:        block.ReceiptsRoot,
		ChildBlocksRootHash: block.ChildBlocksRootHash,
		ShardId:             shardId,
		Transactions:        transactionsRes,
		TransactionHashes:   transactionHashesRes,
		ChildBlocks:         childBlocks,
		MainShardHash:       block.MainShardHash,
		DbTimestamp:         dbTimestamp,
		BaseFee:             block.BaseFee,
		LogsBloom:           bloom,
		L1Number:            block.L1BlockNumber,
		GasUsed:             block.GasUsed,
	}, nil
}

func NewRPCLog(
	log *types.Log, blockId types.BlockNumber,
) *RPCLog {
	if log == nil {
		return nil
	}

	return &RPCLog{log, blockId}
}

func NewRPCReceipt(info *rawapitypes.ReceiptInfo) (*RPCReceipt, error) {
	if info == nil {
		return nil, nil
	}

	receipt := &types.Receipt{}
	if err := receipt.UnmarshalSSZ(info.ReceiptSSZ); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}

	logs := make([]*RPCLog, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs[i] = NewRPCLog(log, info.BlockId)
	}

	debugLogs := make([]*RPCDebugLog, len(receipt.DebugLogs))
	for i, log := range receipt.DebugLogs {
		debugLogs[i] = &RPCDebugLog{Message: string(log.Message), Data: log.Data}
	}

	outReceipts := make([]*RPCReceipt, len(info.OutReceipts))
	for i, outReceipt := range info.OutReceipts {
		var err error
		outReceipts[i], err = NewRPCReceipt(outReceipt)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %d out receipt: %w", i, err)
		}
	}

	res := &RPCReceipt{
		Flags:           info.Flags,
		Success:         receipt.Success,
		Status:          receipt.Status.String(),
		FailedPc:        uint(receipt.FailedPc),
		GasUsed:         receipt.GasUsed,
		Forwarded:       receipt.Forwarded,
		GasPrice:        info.GasPrice,
		Logs:            logs,
		DebugLogs:       debugLogs,
		OutTransactions: info.OutTransactions,
		OutReceipts:     outReceipts,
		TxnHash:         receipt.TxnHash,
		ContractAddress: receipt.ContractAddress,
		BlockHash:       info.BlockHash,
		BlockNumber:     info.BlockId,
		TxnIndex:        info.Index,
		ShardId:         types.ShardIdFromHash(receipt.TxnHash),
		Temporary:       info.Temporary,
		ErrorMessage:    info.ErrorMessage,
		IncludedInMain:  info.IncludedInMain,
	}

	// Set only non-empty bloom
	if len(receipt.Logs) > 0 {
		res.Bloom = types.CreateBloom(types.Receipts{receipt}).Bytes()
	}

	return res, nil
}

// @component DebugRPCContract debugRpcContract object "The debug contract whose structure is requested."
// @componentprop Code HEX-encoded contract code
// @componentprop Contract serialized types.SmartContract structure
// @componentprop Proof serialized data for MPT access operation proving
// @componentprop Storage storage slice of key-value pairs of the data in storage
type DebugRPCContract struct {
	Code         hexutil.Bytes                                 `json:"code"`
	Contract     hexutil.Bytes                                 `json:"contract"`
	Proof        hexutil.Bytes                                 `json:"proof"`
	Storage      map[common.Hash]types.Uint256                 `json:"storage,omitempty"`
	Tokens       map[types.TokenId]types.Value                 `json:"tokens"`
	AsyncContext map[types.TransactionIndex]types.AsyncContext `json:"asyncContext"`
}

// @component OutTransaction outTransaction object "Outbound transaction produced by eth_call and result of its execution."
// @componentprop Transaction transaction object true "Transaction data"
// @componentprop Data data string false "Result of VM execution."
// @componentprop CoinsUsed coinsUsed string true "The amount of coins spent on the transaction."
// @componentprop OutTransactions outTransactions array false "Outbound transactions produced by eth_call and result of its execution."
// @componentprop Error error string false "Error produced by the transaction."
type OutTransaction struct {
	Transaction     *types.OutboundTransaction `json:"transaction"`
	Data            hexutil.Bytes              `json:"data,omitempty"`
	CoinsUsed       types.Value                `json:"coinsUsed"`
	OutTransactions []*OutTransaction          `json:"outTransactions,omitempty"`
	Error           string                     `json:"error,omitempty"`
	Logs            []*types.Log               `json:"logs,omitempty"`
}

func toOutTransactions(input []*rpctypes.OutTransaction) ([]*OutTransaction, error) {
	if len(input) == 0 {
		return nil, nil
	}

	output := make([]*OutTransaction, len(input))
	for i, txn := range input {
		outTxns, err := toOutTransactions(txn.OutTransactions)
		if err != nil {
			return nil, err
		}

		decoded := &types.OutboundTransaction{
			Transaction: &types.Transaction{},
			ForwardKind: txn.ForwardKind,
		}
		if err := decoded.Transaction.UnmarshalSSZ(txn.TransactionSSZ); err != nil {
			return nil, err
		}
		decoded.TxnHash = decoded.Transaction.Hash()

		output[i] = &OutTransaction{
			Transaction:     decoded,
			Data:            txn.Data,
			CoinsUsed:       txn.CoinsUsed,
			OutTransactions: outTxns,
			Error:           txn.Error,
			Logs:            txn.Logs,
		}
	}
	return output, nil
}

// @component CallRes callRes object "Response for eth_call."
// @componentprop Data data string false "Result of VM execution."
// @componentprop CoinsUsed coinsUsed string true "The amount of coins spent on the transaction."
// @componentprop OutTransactions outTransactions array false "Outbound transactions produced by the transaction."
// @componentprop Error error string false "Error produced during the call."
// @componentprop StateOverrides stateOverrides object false "Updated contracts state."
type CallRes struct {
	Data            hexutil.Bytes     `json:"data,omitempty"`
	CoinsUsed       types.Value       `json:"coinsUsed"`
	OutTransactions []*OutTransaction `json:"outTransactions,omitempty"`
	Error           string            `json:"error,omitempty"`
	Logs            []*types.Log      `json:"logs,omitempty"`
	DebugLogs       []*RPCDebugLog    `json:"debugLogs,omitempty"`
	StateOverrides  StateOverrides    `json:"stateOverrides,omitempty"`
}

func toCallRes(input *rpctypes.CallResWithGasPrice) (*CallRes, error) {
	var err error
	output := &CallRes{}
	output.Data = input.Data
	output.CoinsUsed = input.CoinsUsed
	output.Error = input.Error
	output.StateOverrides = input.StateOverrides
	output.OutTransactions, err = toOutTransactions(input.OutTransactions)
	output.Logs = input.Logs

	output.DebugLogs = make([]*RPCDebugLog, len(input.DebugLogs))
	for i, log := range input.DebugLogs {
		output.DebugLogs[i] = &RPCDebugLog{Message: string(log.Message), Data: log.Data}
	}

	return output, err
}

type EstimateFeeRes struct {
	FeeCredit          types.Value `json:"feeCredit"`
	AveragePriorityFee types.Value `json:"averagePriorityFee"`
	MaxBasFee          types.Value `json:"maxBaseFee"`
}
