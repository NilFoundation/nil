package types

import (
	"github.com/NilFoundation/nil/nil/common/sszx"
)

type ReceiptWithSSZ struct {
	Decoded    *Receipt
	SszEncoded sszx.SSZEncodedData
}

type BlockWithSSZ struct {
	Decoded    *BlockWithShardId
	SszEncoded *RawBlockWithExtractedData
}

type BlockWithShardId struct {
	*BlockWithExtractedData
	ShardId ShardId
}

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path indexer.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,../../common/hash.go,block.go,uint256.go --objs ReceiptWithSSZ,BlockWithSSZ,BlockWithShardId
