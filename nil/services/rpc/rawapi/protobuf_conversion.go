package rawapi

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/holiman/uint256"
)

func toPbHash(hash common.Hash) *pb.Hash {
	u256 := hash.Uint256()
	return &pb.Hash{
		P0: u256[0],
		P1: u256[1],
		P2: u256[2],
		P3: u256[3],
	}
}

func fromPbHash(hash *pb.Hash) common.Hash {
	u256 := uint256.Int([4]uint64{hash.P0, hash.P1, hash.P2, hash.P3})
	return common.BytesToHash(u256.Bytes())
}

func (br BlockReference) toPb() *pb.BlockReference {
	return &pb.BlockReference{
		Hash:            toPbHash(br.hash),
		BlockIdentifier: int64(br.blockIdentifier),
		Flags:           br.flags,
	}
}

func (br *BlockReference) fromPb(reference *pb.BlockReference) {
	br.hash = fromPbHash(reference.GetHash())
	br.blockIdentifier = blockIdentifier(reference.BlockIdentifier)
	br.flags = reference.Flags
}

func fromPbBlockReference(reference *pb.BlockReference) BlockReference {
	var ref BlockReference
	ref.fromPb(reference)
	return ref
}

func makePbBlockRequest(shardId types.ShardId, reference BlockReference) *pb.BlockRequest {
	return &pb.BlockRequest{
		ShardId:   uint32(shardId),
		Reference: reference.toPb(),
	}
}

func blockReferenceAndShardIdFromPbBlockRequest(request *pb.BlockRequest) (BlockReference, types.ShardId) {
	return fromPbBlockReference(request.GetReference()), types.ShardId(request.ShardId)
}

func toPbError(err error) *pb.Error {
	return &pb.Error{Message: err.Error()}
}

func fromPbError(err *pb.Error) error {
	return errors.New(err.Message)
}

func toPbErrorMap(errors map[common.Hash]string) map[string]*pb.Error {
	result := make(map[string]*pb.Error, len(errors))
	for key, value := range errors {
		result[key.Hex()] = &pb.Error{Message: value}
	}
	return result
}

func fromPbErrorMap(pbErrors map[string]*pb.Error) map[common.Hash]string {
	result := make(map[common.Hash]string, len(pbErrors))
	for key, value := range pbErrors {
		result[common.HexToHash(key)] = value.Message
	}
	return result
}
