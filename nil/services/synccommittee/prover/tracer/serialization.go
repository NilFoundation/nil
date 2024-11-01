package tracer

import (
	"os"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
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
		StackOps:          make([]StackOp, len(traces.StackOps)),
		MemoryOps:         make([]MemoryOp, len(traces.MemoryOps)),
		StorageOps:        make([]StorageOp, len(traces.StorageOps)),
		ContractsBytecode: make(map[types.Address][]byte, len(traces.ContractBytecodes)),
		SlotsChanges:      make([]map[types.Address][]SlotChangeTrace, len(traces.MessageTraces)),
	}

	for i, pbStackOp := range traces.StackOps {
		ep.StackOps[i] = StackOp{
			IsRead: pbStackOp.IsRead,
			Idx:    int(pbStackOp.Index),
			Value:  protoUint256ToUint256(pbStackOp.Value),
			PC:     pbStackOp.Pc,
			MsgId:  uint(pbStackOp.MsgId),
			RwIdx:  uint(pbStackOp.RwIdx),
		}
	}

	for i, pbMemOp := range traces.MemoryOps {
		ep.MemoryOps[i] = MemoryOp{
			IsRead: pbMemOp.IsRead,
			Idx:    int(pbMemOp.Index),
			Value:  pbMemOp.Value[0],
			PC:     pbMemOp.Pc,
			MsgId:  uint(pbMemOp.MsgId),
			RwIdx:  uint(pbMemOp.RwIdx),
		}
	}

	for i, pbStorageOp := range traces.StorageOps {
		ep.StorageOps[i] = StorageOp{
			IsRead: pbStorageOp.IsRead,
			Key:    common.HexToHash(pbStorageOp.Key),
			Value:  protoUint256ToUint256(pbStorageOp.Value),
			PC:     pbStorageOp.Pc,
			MsgId:  uint(pbStorageOp.MsgId),
			RwIdx:  uint(pbStorageOp.RwIdx),
		}
	}

	for messageIdx, pbMessageTrace := range traces.MessageTraces {
		addrToChanges := make(map[types.Address][]SlotChangeTrace, len(pbMessageTrace.StorageTracesByAddress))
		for pbContractAddr, pbStorageTraces := range pbMessageTrace.StorageTracesByAddress {
			storageTraces := make([]SlotChangeTrace, len(pbStorageTraces.SlotsChanges))
			for i, pbSlotChange := range pbStorageTraces.SlotsChanges {
				proof, err := protoToGoProof(pbSlotChange.SszProof)
				if err != nil {
					return nil, err
				}
				storageTraces[i] = SlotChangeTrace{
					Key:         common.HexToHash(pbSlotChange.Key),
					RootBefore:  common.HexToHash(pbSlotChange.RootBefore),
					RootAfter:   common.HexToHash(pbSlotChange.RootAfter),
					ValueBefore: protoUint256ToUint256(pbSlotChange.ValueBefore),
					ValueAfter:  protoUint256ToUint256(pbSlotChange.ValueAfter),
					Proof:       proof,
				}
			}
			addrToChanges[types.HexToAddress(pbContractAddr)] = storageTraces
		}
		ep.SlotsChanges[messageIdx] = addrToChanges
	}

	for pbContractAddr, pbContractBytecode := range traces.ContractBytecodes {
		ep.ContractsBytecode[types.HexToAddress(pbContractAddr)] = pbContractBytecode
	}

	return ep, nil
}

func ToProto(traces *ExecutionTraces) (*pb.ExecutionTraces, error) {
	pbTraces := &pb.ExecutionTraces{
		StackOps:          make([]*pb.StackOp, len(traces.StackOps)),
		MemoryOps:         make([]*pb.MemoryOp, len(traces.MemoryOps)),
		StorageOps:        make([]*pb.StorageOp, len(traces.StorageOps)),
		ContractBytecodes: make(map[string][]byte),
		MessageTraces:     make([]*pb.MessageTraces, len(traces.SlotsChanges)),
	}

	// Convert StackOps
	for i, stackOp := range traces.StackOps {
		pbTraces.StackOps[i] = &pb.StackOp{
			IsRead: stackOp.IsRead,
			Index:  int32(stackOp.Idx),
			Value:  uint256ToProtoUint256(stackOp.Value),
			Pc:     stackOp.PC,
			MsgId:  uint64(stackOp.MsgId),
			RwIdx:  uint64(stackOp.RwIdx),
		}
	}

	// Convert MemoryOps
	for i, memOp := range traces.MemoryOps {
		pbTraces.MemoryOps[i] = &pb.MemoryOp{
			IsRead: memOp.IsRead,
			Index:  int32(memOp.Idx),
			Value:  []byte{memOp.Value},
			Pc:     memOp.PC,
			MsgId:  uint64(memOp.MsgId),
			RwIdx:  uint64(memOp.RwIdx),
		}
	}

	// Convert StorageOps
	for i, storageOp := range traces.StorageOps {
		pbTraces.StorageOps[i] = &pb.StorageOp{
			IsRead: storageOp.IsRead,
			Key:    storageOp.Key.Hex(),
			Value:  uint256ToProtoUint256(storageOp.Value),
			Pc:     storageOp.PC,
			MsgId:  uint64(storageOp.MsgId),
			RwIdx:  uint64(storageOp.RwIdx),
		}
	}

	// Convert SlotsChanges
	for messageIdx, addrToChanges := range traces.SlotsChanges {
		messageTrace := &pb.MessageTraces{
			StorageTracesByAddress: make(map[string]*pb.AdderssSlotsChanges),
		}

		for addr, storageTraces := range addrToChanges {
			pbStorageTraces := &pb.AdderssSlotsChanges{
				SlotsChanges: make([]*pb.SlotChangeTrace, len(storageTraces)),
			}

			for i, slotChange := range storageTraces {
				encodedProof, err := goToProtoProof(slotChange.Proof)
				if err != nil {
					return nil, err
				}

				pbStorageTraces.SlotsChanges[i] = &pb.SlotChangeTrace{
					Key:         slotChange.Key.Hex(),
					RootBefore:  slotChange.RootBefore.Hex(),
					RootAfter:   slotChange.RootAfter.Hex(),
					ValueBefore: uint256ToProtoUint256(slotChange.ValueBefore),
					ValueAfter:  uint256ToProtoUint256(slotChange.ValueAfter),
					SszProof:    encodedProof,
				}
			}
			messageTrace.StorageTracesByAddress[addr.Hex()] = pbStorageTraces
		}
		pbTraces.MessageTraces[messageIdx] = messageTrace
	}

	// Convert ContractsBytecode
	for addr, bytecode := range traces.ContractsBytecode {
		pbTraces.ContractBytecodes[addr.Hex()] = bytecode
	}

	return pbTraces, nil
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

func goToProtoProof(p mpt.Proof) ([]byte, error) {
	encodedProof, err := p.Encode()
	if err != nil {
		return nil, err
	}
	return encodedProof, nil
}

func protoToGoProof(pbProof []byte) (mpt.Proof, error) {
	return mpt.DecodeProof(pbProof)
}
