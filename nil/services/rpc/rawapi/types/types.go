package rawapitypes

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type BlockReferenceType uint8

const blockReferenceTypeMask = 0b11

const (
	HashBlockReference            = BlockReferenceType(0b00)
	NumberBlockReference          = BlockReferenceType(0b01)
	NamedBlockIdentifierReference = BlockReferenceType(0b10)
	_                             = BlockReferenceType(0b11)
)

type BlockNumber uint64

type NamedBlockIdentifier int64

const (
	EarliestBlock = NamedBlockIdentifier(0)
	LatestBlock   = NamedBlockIdentifier(-1)
)

// BlockIdentifier unlike BlockNumber contains special “named” values in the negative range for addressing blocks.
type blockIdentifier int64

type BlockReference struct {
	hash            common.Hash
	blockIdentifier blockIdentifier

	flags uint32
}

type BlockRequest struct {
	BlockReference
	ShardId types.ShardId
}

func (br BlockReference) Hash() common.Hash {
	if assert.Enable {
		check.PanicIfNot(br.Type() == HashBlockReference)
	}
	return br.hash
}

func (br BlockReference) Number() BlockNumber {
	if assert.Enable {
		check.PanicIfNot(br.Type() == NumberBlockReference)
	}
	return BlockNumber(br.blockIdentifier)
}

func (br BlockReference) NamedBlockIdentifier() NamedBlockIdentifier {
	if assert.Enable {
		check.PanicIfNot(br.Type() == NamedBlockIdentifierReference)
	}
	return NamedBlockIdentifier(br.blockIdentifier)
}

func (br BlockReference) Type() BlockReferenceType {
	return BlockReferenceType(br.flags & blockReferenceTypeMask)
}

func BlockHashAsBlockReference(hash common.Hash) BlockReference {
	return BlockReference{hash: hash, flags: uint32(HashBlockReference)}
}

func BlockNumberAsBlockReference(number types.BlockNumber) BlockReference {
	return BlockReference{blockIdentifier: blockIdentifier(number), flags: uint32(NumberBlockReference)}
}

func NamedBlockIdentifierAsBlockReference(identifier NamedBlockIdentifier) BlockReference {
	return BlockReference{blockIdentifier: blockIdentifier(identifier), flags: uint32(NamedBlockIdentifierReference)}
}
