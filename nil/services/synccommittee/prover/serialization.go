package prover

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	pb "github.com/NilFoundation/nil/nil/services/synccommittee/prover/proto"
	"google.golang.org/protobuf/proto"
)

func SerializeToFile(proofs *ExecutionTraces, filename string) error {
	// Convert ExecutionTraces to protobuf message
	pbTraces, err := ToProto(proofs)
	if err != nil {
		return err
	}

	// Marshal the protobuf message
	data, err := proto.Marshal(pbTraces)
	if err != nil {
		return err
	}

	// Write the marshaled data to file
	return os.WriteFile(filename, data, 0o600)
}

func DeserializeFromFile(filename string) (*ExecutionTraces, error) {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Unmarshal the protobuf message
	var pbTraces pb.ExecutionTraces
	err = proto.Unmarshal(data, &pbTraces)
	if err != nil {
		return nil, err
	}

	// Convert protobuf message back to ExecutionTraces
	return FromProto(&pbTraces)
}

func FromProto(traces *pb.ExecutionTraces) (*ExecutionTraces, error) {
	ep := &ExecutionTraces{
		StackOps:        make([]StackOp, len(traces.StackOps)),
		MemoryOps:       make([]MemoryOp, len(traces.MemoryOps)),
		StorageProofs:   make(map[types.Address][]mpt.Proof),
		ExecutedOpCodes: make([]vm.OpCode, len(traces.ExecutedOpCodes)),
	}

	for i, pbStackOp := range traces.StackOps {
		ep.StackOps[i] = StackOp{
			IsRead: pbStackOp.IsRead,
			Idx:    int(pbStackOp.Index),
			Value:  protoUint256ToUint256(pbStackOp.Value),
			OpCode: vm.OpCode(pbStackOp.OpCode),
		}
	}

	for i, pbMemOp := range traces.MemoryOps {
		ep.MemoryOps[i] = MemoryOp{
			IsRead: pbMemOp.IsRead,
			Idx:    int(pbMemOp.Index),
			Value:  pbMemOp.Value[0],
			OpCode: vm.OpCode(pbMemOp.OpCode),
		}
	}

	for addrHex, pbTraces := range traces.StorageProofsByAddress {
		var addr types.Address
		addrBytes, err := hex.DecodeString(addrHex)
		if err != nil {
			return nil, fmt.Errorf("failed to decode address: %w", err)
		}
		copy(addr[:], addrBytes)

		proofs := make([]mpt.Proof, len(pbTraces.ProofList))
		for i, pbProof := range pbTraces.ProofList {
			proofs[i], err = protoToGoProof(pbProof)
			if err != nil {
				return nil, err
			}
		}
		ep.StorageProofs[addr] = proofs
	}

	for i, pbExecutedOpCode := range traces.ExecutedOpCodes {
		ep.ExecutedOpCodes[i] = vm.OpCode(pbExecutedOpCode)
	}

	return ep, nil
}

func ToProto(proofs *ExecutionTraces) (*pb.ExecutionTraces, error) {
	pbEP := &pb.ExecutionTraces{
		StackOps:               make([]*pb.StackOp, len(proofs.StackOps)),
		MemoryOps:              make([]*pb.MemoryOp, len(proofs.MemoryOps)),
		StorageProofsByAddress: make(map[string]*pb.StorageProofs),
		ExecutedOpCodes:        make([]pb.OpCode, len(proofs.ExecutedOpCodes)),
	}

	for i, sp := range proofs.StackOps {
		pbEP.StackOps[i] = &pb.StackOp{
			IsRead: sp.IsRead,
			Index:  int32(sp.Idx),
			Value:  uint256ToProtoUint256(sp.Value),
			OpCode: pb.OpCode(sp.OpCode),
		}
	}

	for i, mp := range proofs.MemoryOps {
		pbEP.MemoryOps[i] = &pb.MemoryOp{
			IsRead: mp.IsRead,
			Index:  int32(mp.Idx),
			Value:  []byte{mp.Value},
			OpCode: pb.OpCode(mp.OpCode),
		}
	}

	var err error
	for addr, proofs := range proofs.StorageProofs {
		pbProofs := make([]*pb.Proof, len(proofs))
		for i, proof := range proofs {
			pbProofs[i], err = goToProtoProof(proof)
			if err != nil {
				return nil, err
			}
		}
		// Use hex encoding for the address
		addrHex := hex.EncodeToString(addr[:])
		pbEP.StorageProofsByAddress[addrHex] = &pb.StorageProofs{ProofList: pbProofs}
	}

	for i, opCode := range proofs.ExecutedOpCodes {
		pbEP.ExecutedOpCodes[i] = pb.OpCode(opCode)
	}

	return pbEP, nil
}

func uint256ToProtoUint256(u types.Uint256) *pb.Uint256 {
	return &pb.Uint256{
		WordParts: u[:],
	}
}

func protoUint256ToUint256(pb *pb.Uint256) types.Uint256 {
	var u types.Uint256
	copy(u[:], pb.WordParts)
	return u
}

func goToProtoProof(p mpt.Proof) (*pb.Proof, error) {
	encodedProof, err := p.Encode()
	if err != nil {
		return nil, err
	}
	return &pb.Proof{
		ProofData: encodedProof,
	}, nil
}

func protoToGoProof(pbProof *pb.Proof) (mpt.Proof, error) {
	return mpt.DecodeProof(pbProof.ProofData)
}
