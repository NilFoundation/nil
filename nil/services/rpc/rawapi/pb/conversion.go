package pb

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

var Logger = logging.NewLogger("pb_conversion")

// Hash converters

func (h *Hash) UnpackProtoMessage() (common.Hash, error) {
	if h == nil {
		return common.EmptyHash, nil
	}
	u256 := h.Data.UnpackProtoMessage()
	return common.BytesToHash(u256.Bytes()), nil
}

func (h *Hash) PackProtoMessage(hash common.Hash) error {
	h.Data = new(Uint256).PackProtoMessage(types.Uint256(*hash.Uint256()))
	return nil
}

// Uint256 converters

func (u *Uint256) UnpackProtoMessage() types.Uint256 {
	return types.Uint256([4]uint64{u.P0, u.P1, u.P2, u.P3})
}

func (u *Uint256) PackProtoMessage(u256 types.Uint256) *Uint256 {
	u.P0 = u256[0]
	u.P1 = u256[1]
	u.P2 = u256[2]
	u.P3 = u256[3]
	return u
}

// BlockReference converters

func (nbr *NamedBlockReference) UnpackProtoMessage() (rawapitypes.NamedBlockIdentifier, error) {
	switch *nbr {
	case NamedBlockReference_EarliestBlock:
		return rawapitypes.EarliestBlock, nil

	case NamedBlockReference_LatestBlock:
		return rawapitypes.LatestBlock, nil

	case NamedBlockReference_PendingBlock:
		return rawapitypes.PendingBlock, nil

	case NamedBlockReference_UnknownNamedRefType:
		fallthrough
	default:
		return 0, errors.New("unexpected named block reference type")
	}
}

func (nbr *NamedBlockReference) PackProtoMessage(namedBlockIdentifier rawapitypes.NamedBlockIdentifier) error {
	switch namedBlockIdentifier {
	case rawapitypes.EarliestBlock:
		*nbr = NamedBlockReference_EarliestBlock

	case rawapitypes.LatestBlock:
		*nbr = NamedBlockReference_LatestBlock

	case rawapitypes.PendingBlock:
		*nbr = NamedBlockReference_PendingBlock

	default:
		return errors.New("unexpected named block reference type")
	}
	return nil
}

func (br *BlockReference) UnpackProtoMessage() (rawapitypes.BlockReference, error) {
	switch br.Reference.(type) {
	case *BlockReference_Hash:
		hash, err := br.GetHash().UnpackProtoMessage()
		return rawapitypes.BlockHashAsBlockReference(hash), err

	case *BlockReference_BlockIdentifier:
		return rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(br.GetBlockIdentifier())), nil

	case *BlockReference_NamedBlockReference:
		nbr := br.GetNamedBlockReference()
		namedBlockReference, err := nbr.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.BlockReference{}, err
		}
		return rawapitypes.NamedBlockIdentifierAsBlockReference(namedBlockReference), nil

	default:
		return rawapitypes.BlockReference{}, errors.New("unexpected block reference type")
	}
}

func (br *BlockReference) PackProtoMessage(blockReference rawapitypes.BlockReference) error {
	switch blockReference.Type() {
	case rawapitypes.HashBlockReference:
		h := &Hash{}
		err := h.PackProtoMessage(blockReference.Hash())
		if err != nil {
			return err
		}
		br.Reference = &BlockReference_Hash{Hash: h}

	case rawapitypes.NumberBlockReference:
		br.Reference = &BlockReference_BlockIdentifier{uint64(blockReference.Number())}

	case rawapitypes.NamedBlockIdentifierReference:
		var nbr NamedBlockReference
		if err := nbr.PackProtoMessage(blockReference.NamedBlockIdentifier()); err != nil {
			return err
		}
		br.Reference = &BlockReference_NamedBlockReference{nbr}

	default:
		return errors.New("unexpected block reference type")
	}
	return nil
}

// BlockRequest converters

func (br *BlockRequest) UnpackProtoMessage() (rawapitypes.BlockReference, error) {
	ref, err := br.Reference.UnpackProtoMessage()
	if err != nil {
		return rawapitypes.BlockReference{}, err
	}
	return ref, nil
}

func (br *BlockRequest) PackProtoMessage(blockReference rawapitypes.BlockReference) error {
	br.Reference = &BlockReference{}
	return br.Reference.PackProtoMessage(blockReference)
}

// AccountRequest

func (a *Address) UnpackProtoMessage() types.Address {
	var bytes [20]byte
	binary.BigEndian.PutUint32(bytes[:4], a.P0)
	binary.BigEndian.PutUint32(bytes[4:8], a.P1)
	binary.BigEndian.PutUint32(bytes[8:12], a.P2)
	binary.BigEndian.PutUint32(bytes[12:16], a.P3)
	binary.BigEndian.PutUint32(bytes[16:20], a.P4)
	return types.BytesToAddress(bytes[:])
}

func (ar *AccountRequest) UnpackProtoMessage() (types.Address, rawapitypes.BlockReference, error) {
	blockReference, err := ar.BlockReference.UnpackProtoMessage()
	if err != nil {
		return types.EmptyAddress, rawapitypes.BlockReference{}, err
	}

	return ar.Address.UnpackProtoMessage(), blockReference, nil
}

func (a *Address) PackProtoMessage(address types.Address) *Address {
	a.P0 = binary.BigEndian.Uint32(address[:4])
	a.P1 = binary.BigEndian.Uint32(address[4:8])
	a.P2 = binary.BigEndian.Uint32(address[8:12])
	a.P3 = binary.BigEndian.Uint32(address[12:16])
	a.P4 = binary.BigEndian.Uint32(address[16:20])
	return a
}

func (ar *AccountRequest) PackProtoMessage(address types.Address, blockReference rawapitypes.BlockReference) error {
	ar.Address = new(Address).PackProtoMessage(address)
	ar.BlockReference = &BlockReference{}
	return ar.BlockReference.PackProtoMessage(blockReference)
}

// Error converters

func (e *Error) UnpackProtoMessage() error {
	if e.Message == db.ErrKeyNotFound.Error() {
		return db.ErrKeyNotFound
	}
	return errors.New(e.Message)
}

func (e *Error) PackProtoMessage(err error) *Error {
	e.Message = err.Error()
	return e
}

// Map of Errors converters

func packErrorMap(errors map[common.Hash]string) map[string]*Error {
	result := make(map[string]*Error, len(errors))
	for key, value := range errors {
		if !utf8.ValidString(value) {
			Logger.Error().
				Stringer("key", key).
				Hex("value", []byte(value)).
				Msg("invalid UTF-8 string in error map")
			value = "<invalid UTF-8 string>"
		}
		result[key.String()] = &Error{Message: value}
	}
	return result
}

func unpackErrorMap(pbErrors map[string]*Error) map[common.Hash]string {
	result := make(map[common.Hash]string, len(pbErrors))
	for key, value := range pbErrors {
		result[common.HexToHash(key)] = value.Message
	}
	return result
}

// RawBlock converters

func (rb *RawBlock) PackProtoMessage(block sszx.SSZEncodedData) error {
	if block == nil {
		return errors.New("block should not be nil")
	}
	*rb = RawBlock{
		BlockSSZ: block,
	}
	return nil
}

func (rb *RawBlock) UnpackProtoMessage() (sszx.SSZEncodedData, error) {
	return rb.BlockSSZ, nil
}

// RawBlockResponse converters

func (br *RawBlockResponse) PackProtoMessage(block sszx.SSZEncodedData, err error) error {
	if err != nil {
		br.fromError(err)
	} else {
		br.fromData(block)
	}
	return nil
}

func (br *RawBlockResponse) fromError(err error) {
	br.Result = &RawBlockResponse_Error{Error: new(Error).PackProtoMessage(err)}
}

func (br *RawBlockResponse) fromData(data sszx.SSZEncodedData) {
	var rawBlock RawBlock
	if err := rawBlock.PackProtoMessage(data); err != nil {
		br.fromError(err)
	} else {
		br.Result = &RawBlockResponse_Data{Data: &rawBlock}
	}
}

func (br *RawBlockResponse) UnpackProtoMessage() (sszx.SSZEncodedData, error) {
	switch br.Result.(type) {
	case *RawBlockResponse_Error:
		return nil, br.GetError().UnpackProtoMessage()

	case *RawBlockResponse_Data:
		return br.GetData().UnpackProtoMessage()

	default:
		return nil, errors.New("unexpected response")
	}
}

// RawFullBlock converters

func (rb *RawFullBlock) PackProtoMessage(block *types.RawBlockWithExtractedData) error {
	if block == nil {
		return errors.New("block should not be nil")
	}

	*rb = RawFullBlock{
		BlockSSZ:           block.Block,
		InTransactionsSSZ:  block.InTransactions,
		OutTransactionsSSZ: block.OutTransactions,
		ReceiptsSSZ:        block.Receipts,
		Errors:             packErrorMap(block.Errors),
		ChildBlocks:        PackHashes(block.ChildBlocks),
		DbTimestamp:        block.DbTimestamp,
		Config:             block.Config,
	}
	return nil
}

func UnpackHashes(input []*Hash) []common.Hash {
	hashes := make([]common.Hash, len(input))
	for i, hash := range input {
		var err error
		hashes[i], err = hash.UnpackProtoMessage()
		check.PanicIfErr(err)
	}
	return hashes
}

func PackHashes(input []common.Hash) []*Hash {
	hashes := make([]*Hash, len(input))
	for i, hash := range input {
		hashes[i] = &Hash{}
		err := hashes[i].PackProtoMessage(hash)
		check.PanicIfErr(err)
	}
	return hashes
}

func (rb *RawFullBlock) UnpackProtoMessage() (*types.RawBlockWithExtractedData, error) {
	return &types.RawBlockWithExtractedData{
		Block:           rb.BlockSSZ,
		InTransactions:  rb.InTransactionsSSZ,
		OutTransactions: rb.OutTransactionsSSZ,
		Receipts:        rb.ReceiptsSSZ,
		Errors:          unpackErrorMap(rb.Errors),
		ChildBlocks:     UnpackHashes(rb.ChildBlocks),
		DbTimestamp:     rb.DbTimestamp,
		Config:          rb.Config,
	}, nil
}

// RawFullBlockResponse converters

func (br *RawFullBlockResponse) PackProtoMessage(block *types.RawBlockWithExtractedData, err error) error {
	if err != nil {
		br.fromError(err)
	} else {
		br.fromData(block)
	}
	return nil
}

func (br *RawFullBlockResponse) fromError(err error) {
	br.Result = &RawFullBlockResponse_Error{Error: new(Error).PackProtoMessage(err)}
}

func (br *RawFullBlockResponse) fromData(data *types.RawBlockWithExtractedData) {
	var rawBlock RawFullBlock
	if err := rawBlock.PackProtoMessage(data); err != nil {
		br.fromError(err)
	} else {
		br.Result = &RawFullBlockResponse_Data{Data: &rawBlock}
	}
}

func (br *RawFullBlockResponse) UnpackProtoMessage() (*types.RawBlockWithExtractedData, error) {
	switch br.Result.(type) {
	case *RawFullBlockResponse_Error:
		return nil, br.GetError().UnpackProtoMessage()

	case *RawFullBlockResponse_Data:
		return br.GetData().UnpackProtoMessage()

	default:
		return nil, errors.New("unexpected response type")
	}
}

// Uint64Response converters
func (br *Uint64Response) PackProtoMessage(count uint64, err error) error {
	br.Result = &Uint64Response_Count{Count: count}
	if err != nil {
		br.Result = &Uint64Response_Error{Error: new(Error).PackProtoMessage(err)}
	}
	return nil
}

func (br *Uint64Response) UnpackProtoMessage() (uint64, error) {
	switch br.Result.(type) {
	case *Uint64Response_Error:
		return 0, br.GetError().UnpackProtoMessage()
	case *Uint64Response_Count:
		return br.GetCount(), nil
	default:
		return 0, errors.New("unexpected response type")
	}
}

// StringResponse converters
func (br *StringResponse) PackProtoMessage(value string, err error) error {
	br.Result = &StringResponse_Value{Value: value}
	if err != nil {
		br.Result = &StringResponse_Error{Error: new(Error).PackProtoMessage(err)}
	}
	return nil
}

func (br *StringResponse) UnpackProtoMessage() (string, error) {
	switch br.Result.(type) {
	case *StringResponse_Error:
		return "", br.GetError().UnpackProtoMessage()
	case *StringResponse_Value:
		return br.GetValue(), nil
	default:
		return "", errors.New("unexpected response type")
	}
}

func (br *BalanceResponse) PackProtoMessage(balance types.Value, err error) error {
	if err != nil {
		br.Result = &BalanceResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	if balance.Uint256 == nil {
		balance.Uint256 = new(types.Uint256)
	}
	br.Result = &BalanceResponse_Data{Data: new(Uint256).PackProtoMessage(*balance.Uint256)}
	return nil
}

func (br *BalanceResponse) UnpackProtoMessage() (types.Value, error) {
	switch br.Result.(type) {
	case *BalanceResponse_Error:
		return types.Value{}, br.GetError().UnpackProtoMessage()

	case *BalanceResponse_Data:
		return newValueFromUint256(br.GetData()), nil

	default:
		return types.Value{}, errors.New("unexpected response type")
	}
}

// CodeResponse converters
func (br *CodeResponse) PackProtoMessage(code types.Code, err error) error {
	if err != nil {
		br.Result = &CodeResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	br.Result = &CodeResponse_Data{Data: code}
	return nil
}

func (br *CodeResponse) UnpackProtoMessage() (types.Code, error) {
	switch br.Result.(type) {
	case *CodeResponse_Error:
		return nil, br.GetError().UnpackProtoMessage()

	case *CodeResponse_Data:
		return br.GetData(), nil
	}
	return nil, errors.New("unexpected response type")
}

// TokenResponse converters
func (cr *TokensResponse) PackProtoMessage(tokens map[types.TokenId]types.Value, err error) error {
	if err != nil {
		cr.Result = &TokensResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	result := Tokens{Data: make(map[string]*Uint256)}
	for k, v := range tokens {
		result.Data[k.String()] = new(Uint256).PackProtoMessage(*v.Uint256)
	}
	cr.Result = &TokensResponse_Data{Data: &result}
	return nil
}

func (cr *TokensResponse) UnpackProtoMessage() (map[types.TokenId]types.Value, error) {
	switch cr.Result.(type) {
	case *TokensResponse_Error:
		return nil, cr.GetError().UnpackProtoMessage()

	case *TokensResponse_Data:
		data := cr.GetData().Data
		result := make(map[types.TokenId]types.Value, len(data))
		for k, v := range data {
			tokenId := types.TokenId(types.HexToAddress(k))
			result[tokenId] = newValueFromUint256(v)
		}
		return result, nil
	}
	return nil, errors.New("unexpected response type")
}

// AsyncContext converters

func (ac *AsyncContext) PackProtoMessage(context *types.AsyncContext) {
	if context == nil {
		return
	}

	ac.IsAwait = context.IsAwait
	ac.Data = context.Data
	ac.ResponseProcessingGas = context.ResponseProcessingGas.Uint64()
}

func (rc *AsyncContext) UnpackProtoMessage() types.AsyncContext {
	if rc == nil {
		return types.AsyncContext{}
	}
	return types.AsyncContext{
		IsAwait:               rc.IsAwait,
		Data:                  rc.Data,
		ResponseProcessingGas: types.Gas(rc.ResponseProcessingGas),
	}
}

// RawContract converters

func (rc *RawContract) PackProtoMessage(contract *rawapitypes.SmartContract) error {
	rc.ContractSSZ = contract.ContractSSZ
	rc.Code = contract.Code
	rc.ProofEncoded = contract.ProofEncoded

	if contract.Storage != nil {
		rc.Storage = make(map[string]*Uint256)
		for k, v := range contract.Storage {
			rc.Storage[k.Hex()] = new(Uint256).PackProtoMessage(v)
		}
	}

	if contract.Tokens != nil {
		rc.Tokens = make(map[string]*Uint256)
		for k, v := range contract.Tokens {
			u := new(Uint256)
			if v.Uint256 != nil {
				u = u.PackProtoMessage(*v.Uint256)
			}
			rc.Tokens[k.String()] = u
		}
	}

	if contract.AsyncContext != nil {
		rc.AsyncContext = make(map[uint64]*AsyncContext)
		for k, v := range contract.AsyncContext {
			rc.AsyncContext[uint64(k)] = new(AsyncContext)
			rc.AsyncContext[uint64(k)].PackProtoMessage(&v)
		}
	}

	return nil
}

func (rc *RawContract) UnpackProtoMessage() (*rawapitypes.SmartContract, error) {
	contract := &rawapitypes.SmartContract{
		ContractSSZ:  rc.ContractSSZ,
		Code:         rc.Code,
		ProofEncoded: rc.ProofEncoded,
	}

	if len(rc.Storage) > 0 {
		storage := make(map[common.Hash]types.Uint256)
		for k, v := range rc.Storage {
			storage[common.HexToHash(k)] = v.UnpackProtoMessage()
		}
		contract.Storage = storage
	}

	if len(rc.Tokens) > 0 {
		tokens := make(map[types.TokenId]types.Value)
		for k, v := range rc.Tokens {
			tokens[types.TokenId(types.HexToAddress(k))] = newValueFromUint256(v)
		}
		contract.Tokens = tokens
	}

	if len(rc.AsyncContext) > 0 {
		asyncContext := make(map[types.TransactionIndex]types.AsyncContext)
		for k, v := range rc.AsyncContext {
			asyncContext[types.TransactionIndex(k)] = v.UnpackProtoMessage()
		}
		contract.AsyncContext = asyncContext
	}

	return contract, nil
}

// RawContractResponse converters

func (rcr *RawContractResponse) PackProtoMessage(contract *rawapitypes.SmartContract, err error) error {
	if err != nil {
		rcr.Result = &RawContractResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	rawContract := new(RawContract)
	if err := rawContract.PackProtoMessage(contract); err != nil {
		return err
	}

	rcr.Result = &RawContractResponse_Data{Data: rawContract}
	return nil
}

func (rcr *RawContractResponse) UnpackProtoMessage() (*rawapitypes.SmartContract, error) {
	switch rcr.Result.(type) {
	case *RawContractResponse_Error:
		return nil, rcr.GetError().UnpackProtoMessage()

	case *RawContractResponse_Data:
		return rcr.GetData().UnpackProtoMessage()
	}
	return nil, errors.New("unexpected response type")
}

func (c *Contract) PackProtoMessage(contract rpctypes.Contract) *Contract {
	if contract.Seqno != nil {
		c.Seqno = (*uint64)(contract.Seqno)
	}
	if contract.ExtSeqno != nil {
		c.ExtSeqno = (*uint64)(contract.ExtSeqno)
	}
	if contract.Code != nil {
		c.Code = *contract.Code
	}
	if contract.Balance != nil {
		balance := new(Uint256)
		if contract.Balance.Uint256 != nil {
			balance.PackProtoMessage(*contract.Balance.Uint256)
		}
		c.Balance = balance
	}
	if contract.State != nil {
		c.State = make(map[string]*Hash)
		for k, v := range *contract.State {
			kHex := k.Hex()
			c.State[kHex] = &Hash{}
			check.PanicIfErr(c.State[kHex].PackProtoMessage(v))
		}
	}
	if contract.StateDiff != nil {
		c.StateDiff = make(map[string]*Hash)
		for k, v := range *contract.StateDiff {
			kHex := k.Hex()
			c.StateDiff[kHex] = &Hash{}
			check.PanicIfErr(c.StateDiff[kHex].PackProtoMessage(v))
		}
	}
	return c
}

func (a *CallArgs) PackProtoMessage(args rpctypes.CallArgs) *CallArgs {
	a.Flags = uint32(args.Flags.Bits)
	if args.From != nil {
		a.From = new(Address).PackProtoMessage(*args.From)
	}
	a.To = new(Address).PackProtoMessage(args.To)
	if args.Fee.FeeCredit.Uint256 != nil {
		a.FeeCredit = new(Uint256).PackProtoMessage(*args.Fee.FeeCredit.Uint256)
	}
	if args.Fee.MaxFeePerGas.Uint256 != nil {
		a.MaxFeePerGas = new(Uint256).PackProtoMessage(*args.Fee.MaxFeePerGas.Uint256)
	}
	if args.Fee.MaxPriorityFeePerGas.Uint256 != nil {
		a.MaxPriorityFeePerGas = new(Uint256).PackProtoMessage(*args.Fee.MaxPriorityFeePerGas.Uint256)
	}
	if args.Value.Uint256 != nil {
		a.Value = new(Uint256).PackProtoMessage(*args.Value.Uint256)
	}
	a.Seqno = args.Seqno.Uint64()
	if args.Data != nil {
		a.Data = *args.Data
	}
	if args.Transaction != nil {
		a.Transaction = *args.Transaction
	}
	a.ChainId = uint64(args.ChainId)
	return a
}

func (o *StateOverrides) PackProtoMessage(overrides *rpctypes.StateOverrides) *StateOverrides {
	if overrides != nil {
		o.Overrides = make(map[string]*Contract)
		for k, v := range *overrides {
			o.Overrides[k.Hex()] = new(Contract).PackProtoMessage(v)
		}
	}
	return o
}

func (brd *BlockReferenceOrHashWithChildren) PackProtoMessage(
	blockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
) error {
	if blockReferenceOrHashWithChildren.IsReference() {
		blockReference := new(BlockReference)
		if err := blockReference.PackProtoMessage(blockReferenceOrHashWithChildren.Reference()); err != nil {
			return err
		}
		brd.BlockReferenceOrHashWithChildren = &BlockReferenceOrHashWithChildren_BlockReference{
			BlockReference: blockReference,
		}
	} else {
		hash, childBlocks := blockReferenceOrHashWithChildren.HashAndChildren()
		blockHashWithChildren := new(BlockHashWithChildren)

		blockHashWithChildren.Hash = new(Hash)
		if err := blockHashWithChildren.Hash.PackProtoMessage(hash); err != nil {
			return err
		}

		for _, childBlock := range childBlocks {
			childBlockHash := new(Hash)
			if err := childBlockHash.PackProtoMessage(childBlock); err != nil {
				return err
			}
			blockHashWithChildren.Children = append(blockHashWithChildren.Children, childBlockHash)
		}
		brd.BlockReferenceOrHashWithChildren = &BlockReferenceOrHashWithChildren_BlockHashWithChildren{
			BlockHashWithChildren: blockHashWithChildren,
		}
	}
	return nil
}

func (brd *BlockReferenceOrHashWithChildren) UnpackProtoMessage() (
	rawapitypes.BlockReferenceOrHashWithChildren, error,
) {
	switch brd.BlockReferenceOrHashWithChildren.(type) {
	case *BlockReferenceOrHashWithChildren_BlockReference:
		blockReference, err := brd.GetBlockReference().UnpackProtoMessage()
		return rawapitypes.BlockReferenceAsBlockReferenceOrHashWithChildren(blockReference), err

	case *BlockReferenceOrHashWithChildren_BlockHashWithChildren:
		blockWithChildren := brd.GetBlockHashWithChildren()
		hash, err := blockWithChildren.Hash.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.BlockReferenceOrHashWithChildren{}, err
		}
		children := make([]common.Hash, len(blockWithChildren.Children))
		for i, child := range blockWithChildren.Children {
			children[i], err = child.UnpackProtoMessage()
			if err != nil {
				return rawapitypes.BlockReferenceOrHashWithChildren{}, err
			}
		}
		return rawapitypes.BlockHashWithChildrenAsBlockReferenceOrHashWithChildren(hash, children), err
	}
	return rawapitypes.BlockReferenceOrHashWithChildren{}, errors.New("unexpected block reference or data type")
}

func (cr *CallRequest) PackProtoMessage(
	args rpctypes.CallArgs,
	mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
	overrides *rpctypes.StateOverrides,
) error {
	cr.Args = new(CallArgs).PackProtoMessage(args)

	cr.MainBlockReferenceOrHashWithChildren = &BlockReferenceOrHashWithChildren{}
	err := cr.MainBlockReferenceOrHashWithChildren.PackProtoMessage(mainBlockReferenceOrHashWithChildren)
	if err != nil {
		return err
	}

	if overrides != nil {
		cr.StateOverrides = new(StateOverrides).PackProtoMessage(overrides)
	}

	return nil
}

func (cr *CallArgs) UnpackProtoMessage() rpctypes.CallArgs {
	args := rpctypes.CallArgs{}
	args.Flags = types.TransactionFlags{BitFlags: types.BitFlags[uint8]{Bits: uint8(cr.Flags)}}
	if cr.From != nil {
		a := cr.From.UnpackProtoMessage()
		args.From = &a
	}
	args.To = cr.To.UnpackProtoMessage()

	args.Fee.FeeCredit = newValueFromUint256(cr.FeeCredit)
	args.Fee.MaxFeePerGas = newValueFromUint256(cr.MaxFeePerGas)
	args.Fee.MaxPriorityFeePerGas = newValueFromUint256(cr.MaxPriorityFeePerGas)
	args.Value = newValueFromUint256(cr.Value)
	args.Seqno = types.Seqno(cr.Seqno)

	if cr.Data != nil {
		args.Data = (*hexutil.Bytes)(&cr.Data)
	}

	if cr.Transaction != nil {
		args.Transaction = (*hexutil.Bytes)(&cr.Transaction)
	}

	args.ChainId = types.ChainId(cr.ChainId)
	return args
}

func (cr *Contract) UnpackProtoMessage() rpctypes.Contract {
	c := rpctypes.Contract{}

	c.Seqno = (*types.Seqno)(cr.Seqno)
	c.ExtSeqno = (*types.Seqno)(cr.ExtSeqno)

	if len(cr.Code) > 0 {
		c.Code = (*hexutil.Bytes)(&cr.Code)
	}

	if cr.Balance != nil {
		v := newValueFromUint256(cr.Balance)
		c.Balance = &v
	}

	if len(cr.State) > 0 {
		m := make(map[common.Hash]common.Hash)
		for k, v := range cr.State {
			var err error
			m[common.HexToHash(k)], err = v.UnpackProtoMessage()
			check.PanicIfErr(err)
		}
		c.State = &m
	}

	if len(cr.StateDiff) > 0 {
		m := make(map[common.Hash]common.Hash)
		for k, v := range cr.StateDiff {
			var err error
			m[common.HexToHash(k)], err = v.UnpackProtoMessage()
			check.PanicIfErr(err)
		}
		c.StateDiff = &m
	}

	return c
}

func (cr *StateOverrides) UnpackProtoMessage() *rpctypes.StateOverrides {
	if cr == nil {
		return nil
	}

	args := make(rpctypes.StateOverrides)
	for k, v := range cr.Overrides {
		args[types.HexToAddress(k)] = v.UnpackProtoMessage()
	}
	return &args
}

func (cr *CallRequest) UnpackProtoMessage() (
	rpctypes.CallArgs,
	rawapitypes.BlockReferenceOrHashWithChildren,
	*rpctypes.StateOverrides,
	error,
) {
	br, err := cr.MainBlockReferenceOrHashWithChildren.UnpackProtoMessage()
	if err != nil {
		return rpctypes.CallArgs{}, rawapitypes.BlockReferenceOrHashWithChildren{}, nil, err
	}
	return cr.Args.UnpackProtoMessage(), br, cr.StateOverrides.UnpackProtoMessage(), nil
}

func (m *OutTransaction) PackProtoMessage(txn *rpctypes.OutTransaction) *OutTransaction {
	coinsUsed := new(Uint256)
	if txn.CoinsUsed.Uint256 != nil {
		coinsUsed.PackProtoMessage(*txn.CoinsUsed.Uint256)
	}

	gasPrice := new(Uint256)
	if txn.BaseFee.Uint256 != nil {
		gasPrice.PackProtoMessage(*txn.BaseFee.Uint256)
	}

	out := &OutTransaction{
		TransactionSSZ: txn.TransactionSSZ,
		ForwardKind:    uint64(txn.ForwardKind),
		Data:           txn.Data,
		CoinsUsed:      coinsUsed,
		GasPrice:       gasPrice,
		Error:          txn.Error,
		Logs:           packLogs(txn.Logs),
		DebugLogs:      packDebugLogs(txn.DebugLogs),
	}

	if len(txn.OutTransactions) > 0 {
		out.OutTransactions = make([]*OutTransaction, len(txn.OutTransactions))
		for i, outTxn := range txn.OutTransactions {
			out.OutTransactions[i] = new(OutTransaction).PackProtoMessage(outTxn)
		}
	}

	return out
}

func newValueFromUint256(v *Uint256) types.Value {
	if v == nil {
		return types.NewZeroValue()
	}
	value := v.UnpackProtoMessage()
	return types.Value{Uint256: &value}
}

func (m *OutTransaction) UnpackProtoMessage() *rpctypes.OutTransaction {
	txn := &rpctypes.OutTransaction{
		TransactionSSZ: m.TransactionSSZ,
		ForwardKind:    types.ForwardKind(m.ForwardKind),
		Data:           m.Data,
		Error:          m.Error,
		Logs:           unpackLogs(m.Logs),
		DebugLogs:      unpackDebugLogs(m.DebugLogs),
	}

	txn.CoinsUsed = newValueFromUint256(m.CoinsUsed)
	txn.BaseFee = newValueFromUint256(m.GasPrice)

	if len(m.OutTransactions) > 0 {
		txn.OutTransactions = make([]*rpctypes.OutTransaction, len(m.OutTransactions))
		for i, outTxn := range m.OutTransactions {
			txn.OutTransactions[i] = outTxn.UnpackProtoMessage()
		}
	}
	return txn
}

func (l *Log) PackProtoMessage(log *types.Log) {
	l.Address = new(Address).PackProtoMessage(log.Address)
	l.Topics = PackHashes(log.Topics)
	l.Data = log.Data
}

func (l *Log) UnpackProtoMessage() *types.Log {
	return &types.Log{
		Address: l.Address.UnpackProtoMessage(),
		Topics:  UnpackHashes(l.Topics),
		Data:    l.Data,
	}
}

func (l *DebugLog) PackProtoMessage(log *types.DebugLog) {
	l.Message = log.Message
	l.Data = make([]*Uint256, len(log.Data))
	for i, data := range log.Data {
		l.Data[i] = new(Uint256).PackProtoMessage(data)
	}
}

func (l *DebugLog) UnpackProtoMessage() *types.DebugLog {
	data := make([]types.Uint256, len(l.Data))
	for i, value := range l.Data {
		data[i] = value.UnpackProtoMessage()
	}
	return &types.DebugLog{
		Message: l.Message,
		Data:    data,
	}
}

func packLogs(logs []*types.Log) []*Log {
	res := make([]*Log, len(logs))
	for i, log := range logs {
		res[i] = new(Log)
		res[i].PackProtoMessage(log)
	}
	return res
}

func unpackLogs(logs []*Log) []*types.Log {
	if logs == nil {
		return nil
	}
	res := make([]*types.Log, len(logs))
	for i, log := range logs {
		res[i] = log.UnpackProtoMessage()
	}
	return res
}

func packDebugLogs(logs []*types.DebugLog) []*DebugLog {
	res := make([]*DebugLog, len(logs))
	for i, log := range logs {
		res[i] = new(DebugLog)
		res[i].PackProtoMessage(log)
	}
	return res
}

func unpackDebugLogs(logs []*DebugLog) []*types.DebugLog {
	if logs == nil {
		return nil
	}
	res := make([]*types.DebugLog, len(logs))
	for i, log := range logs {
		res[i] = log.UnpackProtoMessage()
	}
	return res
}

func (cr *CallResponse) PackProtoMessage(args *rpctypes.CallResWithGasPrice, err error) error {
	if err != nil {
		cr.Result = &CallResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	res := &CallResult{}
	res.Data = args.Data
	res.Logs = packLogs(args.Logs)
	res.DebugLogs = packDebugLogs(args.DebugLogs)

	if args.CoinsUsed.Uint256 != nil {
		res.CoinsUsed = new(Uint256).PackProtoMessage(*args.CoinsUsed.Uint256)
	}

	res.OutTransactions = make([]*OutTransaction, len(args.OutTransactions))
	for i, outTxn := range res.OutTransactions {
		res.OutTransactions[i] = outTxn.PackProtoMessage(args.OutTransactions[i])
	}

	if len(args.Error) > 0 {
		res.Error = &Error{Message: args.Error}
	}
	if args.StateOverrides != nil {
		res.StateOverrides = new(StateOverrides).PackProtoMessage(&args.StateOverrides)
	}

	if args.BaseFee.Uint256 != nil {
		res.GasPrice = new(Uint256).PackProtoMessage(*args.BaseFee.Uint256)
	}

	cr.Result = &CallResponse_Data{Data: res}
	return nil
}

func (cr *CallResponse) UnpackProtoMessage() (*rpctypes.CallResWithGasPrice, error) {
	if err := cr.GetError(); err != nil {
		return nil, err.UnpackProtoMessage()
	}

	data := cr.GetData()
	if data == nil {
		return nil, errors.New("unexpected response type")
	}
	check.PanicIfNot(data != nil)

	res := &rpctypes.CallResWithGasPrice{}
	res.Data = data.Data
	res.CoinsUsed = newValueFromUint256(data.CoinsUsed)
	res.BaseFee = newValueFromUint256(data.GasPrice)
	res.Logs = unpackLogs(data.Logs)
	res.DebugLogs = unpackDebugLogs(data.DebugLogs)

	res.OutTransactions = make([]*rpctypes.OutTransaction, len(data.OutTransactions))
	for i, outTxn := range data.OutTransactions {
		res.OutTransactions[i] = outTxn.UnpackProtoMessage()
	}

	if data.StateOverrides != nil {
		res.StateOverrides = *data.StateOverrides.UnpackProtoMessage()
	}

	if data.Error != nil {
		res.Error = data.Error.Message
	}

	return res, nil
}

// Transaction converters
func (r *TransactionResponse) PackProtoMessage(info *rawapitypes.TransactionInfo, err error) error {
	if err != nil {
		r.Result = &TransactionResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	var hash Hash
	if err := hash.PackProtoMessage(info.BlockHash); err != nil {
		return err
	}

	r.Result = &TransactionResponse_Data{
		Data: &TransactionInfo{
			TransactionSSZ: info.TransactionSSZ,
			ReceiptSSZ:     info.ReceiptSSZ,
			Index:          uint64(info.Index),
			BlockHash:      &hash,
			BlockId:        uint64(info.BlockId),
		},
	}
	return nil
}

func (r *TransactionResponse) UnpackProtoMessage() (*rawapitypes.TransactionInfo, error) {
	switch r.Result.(type) {
	case *TransactionResponse_Error:
		return nil, r.GetError().UnpackProtoMessage()
	case *TransactionResponse_Data:
		data := r.GetData()
		hash, err := data.BlockHash.UnpackProtoMessage()
		if err != nil {
			return nil, err
		}
		return &rawapitypes.TransactionInfo{
			TransactionSSZ: data.TransactionSSZ,
			ReceiptSSZ:     data.ReceiptSSZ,
			Index:          types.TransactionIndex(data.Index),
			BlockHash:      hash,
			BlockId:        types.BlockNumber(data.BlockId),
		}, nil
	}
	return nil, errors.New("unexpected response type")
}

func (r *TransactionRequestByBlockRefAndIndex) PackProtoMessage(
	ref rawapitypes.BlockReference, index types.TransactionIndex,
) error {
	r.BlockRef = &BlockReference{}
	if err := r.BlockRef.PackProtoMessage(ref); err != nil {
		return err
	}
	r.Index = uint64(index)
	return nil
}

func (r *TransactionRequestByBlockRefAndIndex) UnpackProtoMessage() (
	rawapitypes.BlockReference, types.TransactionIndex, error,
) {
	ref, err := r.BlockRef.UnpackProtoMessage()
	if err != nil {
		return rawapitypes.BlockReference{}, 0, err
	}
	return ref, types.TransactionIndex(r.Index), nil
}

func (r *TransactionRequestByHash) PackProtoMessage(hash common.Hash) error {
	r.Hash = &Hash{}
	return r.Hash.PackProtoMessage(hash)
}

func (r *TransactionRequestByHash) UnpackProtoMessage() (common.Hash, error) {
	return r.Hash.UnpackProtoMessage()
}

func (r *TransactionRequest) PackProtoMessage(request rawapitypes.TransactionRequest) error {
	if request.ByHash != nil {
		byHash := &TransactionRequestByHash{}
		if err := byHash.PackProtoMessage(request.ByHash.Hash); err != nil {
			return err
		}
		r.Request = &TransactionRequest_ByHash{
			ByHash: byHash,
		}
	} else {
		byBlockRefAndIndex := &TransactionRequestByBlockRefAndIndex{}
		if err := byBlockRefAndIndex.PackProtoMessage(
			request.ByBlockRefAndIndex.BlockRef,
			request.ByBlockRefAndIndex.Index,
		); err != nil {
			return err
		}
		r.Request = &TransactionRequest_ByBlockRefAndIndex{
			ByBlockRefAndIndex: byBlockRefAndIndex,
		}
	}
	return nil
}

func (r *TransactionRequest) UnpackProtoMessage() (rawapitypes.TransactionRequest, error) {
	byHash := r.GetByHash()
	if byHash != nil {
		hash, err := byHash.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.TransactionRequest{}, err
		}
		return rawapitypes.TransactionRequest{
			ByHash: &rawapitypes.TransactionRequestByHash{Hash: hash},
		}, nil
	}

	byBlockRefAndIndex := r.GetByBlockRefAndIndex()
	if byBlockRefAndIndex != nil {
		ref, index, err := byBlockRefAndIndex.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.TransactionRequest{}, err
		}
		return rawapitypes.TransactionRequest{
			ByBlockRefAndIndex: &rawapitypes.TransactionRequestByBlockRefAndIndex{
				BlockRef: ref,
				Index:    index,
			},
		}, nil
	}
	return rawapitypes.TransactionRequest{}, errors.New("unexpected request type")
}

// Receipt converters
func (r *ReceiptInfo) PackProtoMessage(info *rawapitypes.ReceiptInfo) *ReceiptInfo {
	if info == nil || info.ReceiptSSZ == nil {
		return nil
	}

	var outReceipts []*ReceiptInfo
	if len(info.OutReceipts) > 0 {
		outReceipts = make([]*ReceiptInfo, len(info.OutReceipts))
		for i, outReceipt := range info.OutReceipts {
			if outReceipt != nil {
				outReceipts[i] = new(ReceiptInfo).PackProtoMessage(outReceipt)
			}
		}
	}

	var gp *Uint256
	if info.GasPrice.Uint256 != nil {
		gp = new(Uint256).PackProtoMessage(*info.GasPrice.Uint256)
	}

	blockHash := &Hash{}
	check.PanicIfErr(blockHash.PackProtoMessage(info.BlockHash))

	return &ReceiptInfo{
		Flags:           uint32(info.Flags.Bits),
		ReceiptSSZ:      info.ReceiptSSZ,
		Index:           uint64(info.Index),
		BlockHash:       blockHash,
		BlockId:         uint64(info.BlockId),
		OutTransactions: PackHashes(info.OutTransactions),
		OutReceipts:     outReceipts,
		IncludedInMain:  info.IncludedInMain,
		ErrorMessage:    &Error{Message: info.ErrorMessage},
		GasPrice:        gp,
		Temporary:       info.Temporary,
	}
}

func (r *ReceiptResponse) PackProtoMessage(info *rawapitypes.ReceiptInfo, err error) error {
	if err != nil {
		r.Result = &ReceiptResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	r.Result = &ReceiptResponse_Data{
		Data: new(ReceiptInfo).PackProtoMessage(info),
	}
	return nil
}

func (r *ReceiptInfo) UnpackProtoMessage() *rawapitypes.ReceiptInfo {
	if r == nil || r.ReceiptSSZ == nil {
		return nil
	}

	var outReceipts []*rawapitypes.ReceiptInfo
	if len(r.OutReceipts) > 0 {
		outReceipts = make([]*rawapitypes.ReceiptInfo, len(r.OutReceipts))
		for i, outReceipt := range r.OutReceipts {
			outReceipts[i] = outReceipt.UnpackProtoMessage()
		}
	}

	var errorMessage string
	if r.ErrorMessage != nil {
		errorMessage = r.ErrorMessage.Message
	}

	blockHash, err := r.BlockHash.UnpackProtoMessage()
	check.PanicIfErr(err)
	return &rawapitypes.ReceiptInfo{
		Flags:           types.NewTransactionFlagsFromBits(uint8(r.Flags)),
		ReceiptSSZ:      r.ReceiptSSZ,
		Index:           types.TransactionIndex(r.Index),
		BlockHash:       blockHash,
		BlockId:         types.BlockNumber(r.BlockId),
		OutTransactions: UnpackHashes(r.OutTransactions),
		OutReceipts:     outReceipts,
		IncludedInMain:  r.IncludedInMain,
		ErrorMessage:    errorMessage,
		GasPrice:        newValueFromUint256(r.GasPrice),
		Temporary:       r.Temporary,
	}
}

func (r *ReceiptResponse) UnpackProtoMessage() (*rawapitypes.ReceiptInfo, error) {
	err := r.GetError()
	if err != nil {
		return nil, err.UnpackProtoMessage()
	}
	return r.GetData().UnpackProtoMessage(), nil
}

func (r *GasPriceResponse) PackProtoMessage(v types.Value, err error) error {
	if err != nil {
		r.Result = &GasPriceResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	value := v.Uint256
	if value == nil {
		value = &types.Uint256{}
	}

	r.Result = &GasPriceResponse_Data{Data: new(Uint256).PackProtoMessage(*value)}
	return nil
}

func (r *GasPriceResponse) UnpackProtoMessage() (types.Value, error) {
	err := r.GetError()
	if err != nil {
		return types.Value{}, err.UnpackProtoMessage()
	}
	v := r.GetData()
	if v == nil {
		return types.NewZeroValue(), nil
	}
	return newValueFromUint256(v), nil
}

func (sr *ShardIdListResponse) PackProtoMessage(shardIdList []types.ShardId, err error) error {
	if err != nil {
		sr.Result = &ShardIdListResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	result := &ShardIdList{
		Ids: make([]uint32, 0, len(shardIdList)),
	}
	for _, shardId := range shardIdList {
		result.Ids = append(result.Ids, uint32(shardId))
	}
	sr.Result = &ShardIdListResponse_Data{Data: result}
	return nil
}

func (sr *ShardIdListResponse) UnpackProtoMessage() ([]types.ShardId, error) {
	switch sr.Result.(type) {
	case *ShardIdListResponse_Error:
		return nil, sr.GetError().UnpackProtoMessage()

	case *ShardIdListResponse_Data:
		data := sr.GetData()
		if data == nil {
			return nil, errors.New("unexpected response")
		}

		shardIdList := make([]types.ShardId, 0, len(data.Ids))
		for _, id := range data.Ids {
			shardIdList = append(shardIdList, types.ShardId(id))
		}
		return shardIdList, nil
	}
	return nil, errors.New("unexpected response type")
}

func (r *SendTransactionResponse) PackProtoMessage(status txnpool.DiscardReason, err error) error {
	if err != nil {
		r.Result = &SendTransactionResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	r.Result = &SendTransactionResponse_Status{
		Status: uint32(status),
	}
	return nil
}

func (r *SendTransactionResponse) UnpackProtoMessage() (txnpool.DiscardReason, error) {
	err := r.GetError()
	if err != nil {
		return 0, err.UnpackProtoMessage()
	}

	status := r.GetStatus()
	return txnpool.DiscardReason(status), nil
}

func (r *SendTransactionRequest) PackProtoMessage(transactionSSZ []byte) error {
	r.TransactionSSZ = transactionSSZ
	return nil
}

func (r *SendTransactionRequest) UnpackProtoMessage() ([]byte, error) {
	return r.TransactionSSZ, nil
}
