package pb

import (
	"encoding/binary"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
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

func (ar *AccountRequest) UnpackProtoMessage() (types.Address, rawapitypes.BlockReference, error) {
	blockReference, err := ar.BlockReference.UnpackProtoMessage()
	if err != nil {
		return types.EmptyAddress, rawapitypes.BlockReference{}, err
	}

	var bytes [20]byte
	binary.BigEndian.PutUint32(bytes[:4], ar.Address.P0)
	binary.BigEndian.PutUint32(bytes[4:8], ar.Address.P1)
	binary.BigEndian.PutUint32(bytes[8:12], ar.Address.P2)
	binary.BigEndian.PutUint32(bytes[12:16], ar.Address.P3)
	binary.BigEndian.PutUint32(bytes[16:20], ar.Address.P4)

	return types.BytesToAddress(bytes[:]), blockReference, nil
}

func (ar *AccountRequest) PackProtoMessage(address types.Address, blockReference rawapitypes.BlockReference) error {
	ar.Address = &Address{
		P0: binary.BigEndian.Uint32(address[:4]),
		P1: binary.BigEndian.Uint32(address[4:8]),
		P2: binary.BigEndian.Uint32(address[8:12]),
		P3: binary.BigEndian.Uint32(address[12:16]),
		P4: binary.BigEndian.Uint32(address[16:20]),
	}

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
