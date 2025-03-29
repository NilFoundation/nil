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
	PendingBlock  = NamedBlockIdentifier(-2)
)

// BlockIdentifier unlike BlockNumber contains special “named” values in the negative range for addressing blocks.
type blockIdentifier int64

type BlockReference struct {
	hash            common.Hash
	blockIdentifier blockIdentifier

	flags uint32
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
	check.PanicIfNot(identifier <= 0)
	return BlockReference{blockIdentifier: blockIdentifier(identifier), flags: uint32(NamedBlockIdentifierReference)}
}

type BlockReferenceOrHashWithChildren struct {
	reference BlockReference

	hash        common.Hash
	childBlocks []common.Hash

	isReference bool
}

func (brd BlockReferenceOrHashWithChildren) Reference() BlockReference {
	check.PanicIfNot(brd.isReference)
	return brd.reference
}

func (brd BlockReferenceOrHashWithChildren) HashAndChildren() (common.Hash, []common.Hash) {
	check.PanicIfNot(!brd.isReference)
	return brd.hash, brd.childBlocks
}

func (brd BlockReferenceOrHashWithChildren) IsReference() bool {
	return brd.isReference
}

func BlockReferenceAsBlockReferenceOrHashWithChildren(reference BlockReference) BlockReferenceOrHashWithChildren {
	return BlockReferenceOrHashWithChildren{reference: reference, isReference: true}
}

func BlockHashWithChildrenAsBlockReferenceOrHashWithChildren(hash common.Hash, childBlocks []common.Hash,
) BlockReferenceOrHashWithChildren {
	return BlockReferenceOrHashWithChildren{hash: hash, childBlocks: childBlocks, isReference: false}
}

type TransactionInfo struct {
	TransactionSSZ []byte
	ReceiptSSZ     []byte
	Index          types.TransactionIndex
	BlockHash      common.Hash
	BlockId        types.BlockNumber
}

type ReceiptInfo struct {
	ReceiptSSZ      []byte
	Flags           types.TransactionFlags
	Index           types.TransactionIndex
	BlockHash       common.Hash
	BlockId         types.BlockNumber
	OutTransactions []common.Hash
	OutReceipts     []*ReceiptInfo
	IncludedInMain  bool
	ErrorMessage    string
	GasPrice        types.Value
	Temporary       bool
}

type TransactionRequestByBlockRefAndIndex struct {
	BlockRef BlockReference
	Index    types.TransactionIndex
}

type TransactionRequestByHash struct {
	Hash common.Hash
}

type TransactionRequest struct {
	ByBlockRefAndIndex *TransactionRequestByBlockRefAndIndex
	ByHash             *TransactionRequestByHash
}

type SmartContract struct {
	ContractSSZ  []byte
	Code         types.Code
	ProofEncoded []byte
	Storage      map[common.Hash]types.Uint256
	Tokens       map[types.TokenId]types.Value
	AsyncContext map[types.TransactionIndex]types.AsyncContext
}
