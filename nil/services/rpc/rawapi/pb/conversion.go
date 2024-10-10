package pb

import (
	"encoding/binary"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

// Hash converters

func (h *Hash) UnpackProtoMessage() common.Hash {
	u256 := h.Data.UnpackProtoMessage()
	return common.BytesToHash(u256.Bytes())
}

func (h *Hash) PackProtoMessage(hash common.Hash) *Hash {
	h.Data = new(Uint256).PackProtoMessage(types.Uint256(*hash.Uint256()))
	return h
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

	default:
		return errors.New("unexpected named block reference type")
	}
	return nil
}

func (br *BlockReference) UnpackProtoMessage() (rawapitypes.BlockReference, error) {
	switch br.Reference.(type) {
	case *BlockReference_Hash:
		hash := br.GetHash().UnpackProtoMessage()
		return rawapitypes.BlockHashAsBlockReference(hash), nil

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
		br.Reference = &BlockReference_Hash{new(Hash).PackProtoMessage(blockReference.Hash())}

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
		result[string(key.Bytes())] = &Error{Message: value}
	}
	return result
}

func unpackErrorMap(pbErrors map[string]*Error) map[common.Hash]string {
	result := make(map[common.Hash]string, len(pbErrors))
	for key, value := range pbErrors {
		result[common.BytesToHash([]byte(key))] = value.Message
	}
	return result
}

// RawBlock converters

func (rb *RawBlock) PackProtoMessage(block ssz.SSZEncodedData) error {
	if block == nil {
		return errors.New("block should not be nil")
	}
	*rb = RawBlock{
		BlockSSZ: block,
	}
	return nil
}

func (rb *RawBlock) UnpackProtoMessage() (ssz.SSZEncodedData, error) {
	return rb.BlockSSZ, nil
}

// RawBlockResponse converters

func (br *RawBlockResponse) PackProtoMessage(block ssz.SSZEncodedData, err error) error {
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

func (br *RawBlockResponse) fromData(data ssz.SSZEncodedData) {
	var rawBlock RawBlock
	if err := rawBlock.PackProtoMessage(data); err != nil {
		br.fromError(err)
	} else {
		br.Result = &RawBlockResponse_Data{Data: &rawBlock}
	}
}

func (br *RawBlockResponse) UnpackProtoMessage() (ssz.SSZEncodedData, error) {
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

	childBlocks := make([]*Hash, len(block.ChildBlocks))
	for i, hash := range block.ChildBlocks {
		childBlocks[i] = new(Hash).PackProtoMessage(hash)
	}

	*rb = RawFullBlock{
		BlockSSZ:       block.Block,
		InMessagesSSZ:  block.InMessages,
		OutMessagesSSZ: block.OutMessages,
		ReceiptsSSZ:    block.Receipts,
		Errors:         packErrorMap(block.Errors),
		ChildBlocks:    childBlocks,
		DbTimestamp:    block.DbTimestamp,
	}
	return nil
}

func (rb *RawFullBlock) UnpackProtoMessage() (*types.RawBlockWithExtractedData, error) {
	childBlocks := make([]common.Hash, len(rb.ChildBlocks))
	for i, hash := range rb.ChildBlocks {
		childBlocks[i] = hash.UnpackProtoMessage()
	}

	return &types.RawBlockWithExtractedData{
		Block:       rb.BlockSSZ,
		InMessages:  rb.InMessagesSSZ,
		OutMessages: rb.OutMessagesSSZ,
		Receipts:    rb.ReceiptsSSZ,
		Errors:      unpackErrorMap(rb.Errors),
		ChildBlocks: childBlocks,
		DbTimestamp: rb.DbTimestamp,
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
		u256 := br.GetData().UnpackProtoMessage()
		return types.Value{Uint256: &u256}, nil

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

// CurrencyResponse converters
func (cr *CurrenciesResponse) PackProtoMessage(currencies map[types.CurrencyId]types.Value, err error) error {
	if err != nil {
		cr.Result = &CurrenciesResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	result := Currencies{Data: make(map[string]*Uint256)}
	for k, v := range currencies {
		result.Data[k.String()] = new(Uint256).PackProtoMessage(*v.Uint256)
	}
	cr.Result = &CurrenciesResponse_Data{Data: &result}
	return nil
}

func (cr *CurrenciesResponse) UnpackProtoMessage() (map[types.CurrencyId]types.Value, error) {
	switch cr.Result.(type) {
	case *CurrenciesResponse_Error:
		return nil, cr.GetError().UnpackProtoMessage()

	case *CurrenciesResponse_Data:
		data := cr.GetData().Data
		result := make(map[types.CurrencyId]types.Value, len(data))
		for k, v := range data {
			currencyId := types.CurrencyId(types.HexToAddress(k))
			u256 := v.UnpackProtoMessage()
			result[currencyId] = types.Value{Uint256: &u256}
		}
		return result, nil
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
			c.State[k.Hex()] = new(Hash).PackProtoMessage(v)
		}
	}
	if contract.StateDiff != nil {
		c.StateDiff = make(map[string]*Hash)
		for k, v := range *contract.StateDiff {
			c.StateDiff[k.Hex()] = new(Hash).PackProtoMessage(v)
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
	if args.FeeCredit.Uint256 != nil {
		a.FeeCredit = new(Uint256).PackProtoMessage(*args.FeeCredit.Uint256)
	}
	if args.Value.Uint256 != nil {
		a.Value = new(Uint256).PackProtoMessage(*args.Value.Uint256)
	}
	a.Seqno = args.Seqno.Uint64()
	if args.Data != nil {
		a.Data = *args.Data
	}
	if args.Message != nil {
		a.Message = *args.Message
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

func (cr *CallRequest) PackProtoMessage(
	args rpctypes.CallArgs, mainBlockNrOrHash rawapitypes.BlockReference, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
) error {
	cr.Args = new(CallArgs).PackProtoMessage(args)

	cr.MainBlockNrOrHash = &BlockReference{}
	if err := cr.MainBlockNrOrHash.PackProtoMessage(mainBlockNrOrHash); err != nil {
		return err
	}

	if overrides != nil {
		cr.StateOverrides = new(StateOverrides).PackProtoMessage(overrides)
	}

	cr.EmptyMessageIsRoot = emptyMessageIsRoot
	return nil
}

func (cr *CallArgs) UnpackProtoMessage() rpctypes.CallArgs {
	args := rpctypes.CallArgs{}
	args.Flags = types.MessageFlags{BitFlags: types.BitFlags[uint8]{Bits: uint8(cr.Flags)}}
	if cr.From != nil {
		a := cr.From.UnpackProtoMessage()
		args.From = &a
	}
	args.To = cr.To.UnpackProtoMessage()

	if cr.FeeCredit != nil {
		fc := cr.FeeCredit.UnpackProtoMessage()
		args.FeeCredit = types.Value{Uint256: &fc}
	}

	if cr.Value != nil {
		v := cr.Value.UnpackProtoMessage()
		args.Value = types.Value{Uint256: &v}
	}

	args.Seqno = types.Seqno(cr.Seqno)

	if cr.Data != nil {
		args.Data = (*hexutil.Bytes)(&cr.Data)
	}

	if cr.Message != nil {
		args.Message = (*hexutil.Bytes)(&cr.Message)
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
		v := cr.Balance.UnpackProtoMessage()
		c.Balance = &types.Value{Uint256: &v}
	}

	if len(cr.State) > 0 {
		m := make(map[common.Hash]common.Hash)
		for k, v := range cr.State {
			m[common.HexToHash(k)] = v.UnpackProtoMessage()
		}
		c.State = &m
	}

	if len(cr.StateDiff) > 0 {
		m := make(map[common.Hash]common.Hash)
		for k, v := range cr.StateDiff {
			m[common.HexToHash(k)] = v.UnpackProtoMessage()
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

func (cr *CallRequest) UnpackProtoMessage() (rpctypes.CallArgs, rawapitypes.BlockReference, *rpctypes.StateOverrides, bool, error) {
	br, err := cr.MainBlockNrOrHash.UnpackProtoMessage()
	if err != nil {
		return rpctypes.CallArgs{}, rawapitypes.BlockReference{}, nil, false, err
	}
	return cr.Args.UnpackProtoMessage(), br, cr.StateOverrides.UnpackProtoMessage(), cr.EmptyMessageIsRoot, nil
}

func (m *OutMessage) PackProtoMessage(msg *rpctypes.OutMessage) *OutMessage {
	coinsUsed := new(Uint256)
	if msg.CoinsUsed.Uint256 != nil {
		coinsUsed.PackProtoMessage(*msg.CoinsUsed.Uint256)
	}

	out := &OutMessage{
		MessageSSZ:  msg.MessageSSZ,
		ForwardKind: uint64(msg.ForwardKind),
		Data:        msg.Data,
		CoinsUsed:   coinsUsed,
		Error:       msg.Error,
	}

	if len(msg.OutMessages) > 0 {
		out.OutMessages = make([]*OutMessage, len(msg.OutMessages))
		for i, outMsg := range msg.OutMessages {
			out.OutMessages[i] = new(OutMessage).PackProtoMessage(outMsg)
		}
	}

	return out
}

func (m *OutMessage) UnpackProtoMessage() *rpctypes.OutMessage {
	msg := &rpctypes.OutMessage{
		MessageSSZ:  m.MessageSSZ,
		ForwardKind: types.ForwardKind(m.ForwardKind),
		Data:        m.Data,
		Error:       m.Error,
	}

	if m.CoinsUsed != nil {
		coinsUsed := m.CoinsUsed.UnpackProtoMessage()
		msg.CoinsUsed = types.Value{Uint256: &coinsUsed}
	}

	if len(m.OutMessages) > 0 {
		msg.OutMessages = make([]*rpctypes.OutMessage, len(m.OutMessages))
		for i, outMsg := range m.OutMessages {
			msg.OutMessages[i] = outMsg.UnpackProtoMessage()
		}
	}
	return msg
}

func (cr *CallResponse) PackProtoMessage(args *rpctypes.CallResWithGasPrice, err error) error {
	if err != nil {
		cr.Result = &CallResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	res := &CallResult{}
	res.Data = args.Data

	if args.CoinsUsed.Uint256 != nil {
		res.CoinsUsed = new(Uint256).PackProtoMessage(*args.CoinsUsed.Uint256)
	}

	res.OutMessages = make([]*OutMessage, len(args.OutMessages))
	for i, outMsg := range res.OutMessages {
		res.OutMessages[i] = outMsg.PackProtoMessage(args.OutMessages[i])
	}

	if len(args.Error) > 0 {
		res.Error = &Error{Message: args.Error}
	}
	if args.StateOverrides != nil {
		res.StateOverrides = new(StateOverrides).PackProtoMessage(&args.StateOverrides)
	}

	if args.GasPrice.Uint256 != nil {
		res.GasPrice = new(Uint256).PackProtoMessage(*args.GasPrice.Uint256)
	}

	cr.Result = &CallResponse_Data{Data: res}
	return nil
}

func (cr *CallResponse) UnpackProtoMessage() (*rpctypes.CallResWithGasPrice, error) {
	if err := cr.GetError(); err != nil {
		return nil, err.UnpackProtoMessage()
	}

	data := cr.GetData()
	check.PanicIfNot(data != nil)

	res := &rpctypes.CallResWithGasPrice{}
	res.Data = data.Data

	if data.CoinsUsed != nil {
		value := data.CoinsUsed.UnpackProtoMessage()
		res.CoinsUsed = types.Value{Uint256: &value}
	}

	res.OutMessages = make([]*rpctypes.OutMessage, len(data.OutMessages))
	for i, outMsg := range data.OutMessages {
		res.OutMessages[i] = outMsg.UnpackProtoMessage()
	}

	if data.StateOverrides != nil {
		res.StateOverrides = *data.StateOverrides.UnpackProtoMessage()
	}

	if data.Error != nil {
		res.Error = data.Error.Message
	}

	if data.GasPrice != nil {
		gp := data.GasPrice.UnpackProtoMessage()
		res.GasPrice = types.Value{Uint256: &gp}
	}

	return res, nil
}

// Message converters
func (r *MessageResponse) PackProtoMessage(info *rawapitypes.MessageInfo, err error) error {
	if err != nil {
		r.Result = &MessageResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}
	r.Result = &MessageResponse_Data{
		Data: &MessageInfo{
			MessageSSZ: info.MessageSSZ,
			ReceiptSSZ: info.ReceiptSSZ,
			Index:      uint64(info.Index),
			BlockHash:  new(Hash).PackProtoMessage(info.BlockHash),
			BlockId:    uint64(info.BlockId),
		},
	}
	return nil
}

func (r *MessageResponse) UnpackProtoMessage() (*rawapitypes.MessageInfo, error) {
	switch r.Result.(type) {
	case *MessageResponse_Error:
		return nil, r.GetError().UnpackProtoMessage()
	case *MessageResponse_Data:
		data := r.GetData()
		return &rawapitypes.MessageInfo{
			MessageSSZ: data.MessageSSZ,
			ReceiptSSZ: data.ReceiptSSZ,
			Index:      types.MessageIndex(data.Index),
			BlockHash:  data.BlockHash.UnpackProtoMessage(),
			BlockId:    types.BlockNumber(data.BlockId),
		}, nil
	}
	return nil, errors.New("unexpected response type")
}

func (r *MessageRequestByBlockRefAndIndex) PackProtoMessage(ref rawapitypes.BlockReference, index types.MessageIndex) error {
	r.BlockRef = &BlockReference{}
	if err := r.BlockRef.PackProtoMessage(ref); err != nil {
		return err
	}
	r.Index = uint64(index)
	return nil
}

func (r *MessageRequestByBlockRefAndIndex) UnpackProtoMessage() (rawapitypes.BlockReference, types.MessageIndex, error) {
	ref, err := r.BlockRef.UnpackProtoMessage()
	if err != nil {
		return rawapitypes.BlockReference{}, 0, err
	}
	return ref, types.MessageIndex(r.Index), nil
}

func (r *MessageRequestByHash) PackProtoMessage(hash common.Hash) error {
	r.Hash = new(Hash).PackProtoMessage(hash)
	return nil
}

func (r *MessageRequestByHash) UnpackProtoMessage() (common.Hash, error) {
	return r.Hash.UnpackProtoMessage(), nil
}

func (r *MessageRequest) PackProtoMessage(request rawapitypes.MessageRequest) error {
	if request.ByHash != nil {
		byHash := &MessageRequestByHash{}
		if err := byHash.PackProtoMessage(request.ByHash.Hash); err != nil {
			return err
		}
		r.Request = &MessageRequest_ByHash{
			ByHash: byHash,
		}
	} else {
		byBlockRefAndIndex := &MessageRequestByBlockRefAndIndex{}
		if err := byBlockRefAndIndex.PackProtoMessage(
			request.ByBlockRefAndIndex.BlockRef,
			request.ByBlockRefAndIndex.Index,
		); err != nil {
			return err
		}
		r.Request = &MessageRequest_ByBlockRefAndIndex{
			ByBlockRefAndIndex: byBlockRefAndIndex,
		}
	}
	return nil
}

func (r *MessageRequest) UnpackProtoMessage() (rawapitypes.MessageRequest, error) {
	byHash := r.GetByHash()
	if byHash != nil {
		hash, err := byHash.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.MessageRequest{}, err
		}
		return rawapitypes.MessageRequest{
			ByHash: &rawapitypes.MessageRequestByHash{Hash: hash},
		}, nil
	}

	byBlockRefAndIndex := r.GetByBlockRefAndIndex()
	if byBlockRefAndIndex != nil {
		ref, index, err := byBlockRefAndIndex.UnpackProtoMessage()
		if err != nil {
			return rawapitypes.MessageRequest{}, err
		}
		return rawapitypes.MessageRequest{
			ByBlockRefAndIndex: &rawapitypes.MessageRequestByBlockRefAndIndex{
				BlockRef: ref,
				Index:    index,
			},
		}, nil
	}
	return rawapitypes.MessageRequest{}, errors.New("unexpected request type")
}
