package tracer

import (
	"os"

	"github.com/NilFoundation/nil/nil/common"
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
		StackOps:          make([]StackOp, len(traces.StackOps)),
		MemoryOps:         make([]MemoryOp, len(traces.MemoryOps)),
		StorageOps:        make([]StorageOp, len(traces.StorageOps)),
		ZKEVMStates:       make([]ZKEVMState, len(traces.ZkevmStates)),
		ContractsBytecode: make(map[types.Address][]byte, len(traces.ContractBytecodes)),
		SlotsChanges:      make([]map[types.Address][]SlotChangeTrace, len(traces.MessageTraces)),
		CopyEvents:        make([]CopyEvent, len(traces.CopyEvents)),
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
			IsRead:       pbStorageOp.IsRead,
			Key:          common.HexToHash(pbStorageOp.Key),
			Value:        protoUint256ToUint256(pbStorageOp.Value),
			InitialValue: protoUint256ToUint256(pbStorageOp.InitialValue),
			PC:           pbStorageOp.Pc,
			MsgId:        uint(pbStorageOp.MsgId),
			RwIdx:        uint(pbStorageOp.RwIdx),
			Addr:         types.HexToAddress(pbStorageOp.Address.String()),
		}
	}

	for i, pbZKEVMState := range traces.ZkevmStates {
		ep.ZKEVMStates[i] = ZKEVMState{
			TxHash:          common.HexToHash(pbZKEVMState.TxHash),
			TxId:            int(pbZKEVMState.CallId),
			PC:              pbZKEVMState.Pc,
			Gas:             pbZKEVMState.Gas,
			RwIdx:           uint(pbZKEVMState.RwIdx),
			BytecodeHash:    common.HexToHash(pbZKEVMState.BytecodeHash),
			OpCode:          vm.OpCode(pbZKEVMState.Opcode),
			AdditionalInput: protoUint256ToUint256(pbZKEVMState.AdditionalInput),
			StackSize:       pbZKEVMState.StackSize,
			MemorySize:      pbZKEVMState.MemorySize,
			TxFinish:        pbZKEVMState.TxFinish,
			StackSlice:      make([]types.Uint256, len(pbZKEVMState.StackSlice)),
			MemorySlice:     make(map[uint64]uint8),
			StorageSlice:    make(map[types.Uint256]types.Uint256),
		}

		for j, stackVal := range pbZKEVMState.StackSlice {
			ep.ZKEVMStates[i].StackSlice[j] = protoUint256ToUint256(stackVal)
		}
		for addr, memVal := range pbZKEVMState.MemorySlice {
			ep.ZKEVMStates[i].MemorySlice[addr] = uint8(memVal)
		}
		for _, entry := range pbZKEVMState.StorageSlice {
			key := protoUint256ToUint256(entry.Key)
			ep.ZKEVMStates[i].StorageSlice[key] = protoUint256ToUint256(entry.Value)
		}
	}

	for i, pbCopyEventTrace := range traces.GetCopyEvents() {
		ep.CopyEvents[i].From = copyParticipantFromProto(pbCopyEventTrace.From)
		ep.CopyEvents[i].To = copyParticipantFromProto(pbCopyEventTrace.To)
		ep.CopyEvents[i].RwIdx = uint(pbCopyEventTrace.RwIdx)
		ep.CopyEvents[i].Data = pbCopyEventTrace.GetData()
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
		ZkevmStates:       make([]*pb.ZKEVMState, len(traces.ZKEVMStates)),
		CopyEvents:        make([]*pb.CopyEvent, len(traces.CopyEvents)),
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
			IsRead:       storageOp.IsRead,
			Key:          storageOp.Key.Hex(),
			Value:        uint256ToProtoUint256(storageOp.Value),
			InitialValue: uint256ToProtoUint256(storageOp.InitialValue),
			Pc:           storageOp.PC,
			MsgId:        uint64(storageOp.MsgId),
			RwIdx:        uint64(storageOp.RwIdx),
			Address:      &pb.Address{AddressBytes: storageOp.Addr.Bytes()},
		}
	}

	for i, zkevmState := range traces.ZKEVMStates {
		pbTraces.ZkevmStates[i] = &pb.ZKEVMState{
			TxHash:          zkevmState.TxHash.Hex(),
			CallId:          uint64(zkevmState.TxId),
			Pc:              zkevmState.PC,
			Gas:             zkevmState.Gas,
			RwIdx:           uint64(zkevmState.RwIdx),
			BytecodeHash:    zkevmState.BytecodeHash.String(),
			Opcode:          uint64(zkevmState.OpCode),
			AdditionalInput: uint256ToProtoUint256(zkevmState.AdditionalInput),
			StackSize:       zkevmState.StackSize,
			MemorySize:      zkevmState.MemorySize,
			TxFinish:        zkevmState.TxFinish,
			StackSlice:      make([]*pb.Uint256, len(zkevmState.StackSlice)),
			MemorySlice:     make(map[uint64]uint32),
			StorageSlice:    make([]*pb.StorageEntry, len(zkevmState.StorageSlice)),
		}
		for j, stackVal := range zkevmState.StackSlice {
			pbTraces.ZkevmStates[i].StackSlice[j] = uint256ToProtoUint256(stackVal)
		}
		for addr, memVal := range zkevmState.MemorySlice {
			pbTraces.ZkevmStates[i].MemorySlice[addr] = uint32(memVal)
		}
		storageSliceCounter := 0
		for storageKey, storageVal := range zkevmState.StorageSlice {
			pbEntry := &pb.StorageEntry{
				Key:   uint256ToProtoUint256(storageKey),
				Value: uint256ToProtoUint256(storageVal),
			}
			pbTraces.ZkevmStates[i].StorageSlice[storageSliceCounter] = pbEntry
			storageSliceCounter++
		}
	}

	for i, copyEvent := range traces.CopyEvents {
		pbTraces.CopyEvents[i] = &pb.CopyEvent{
			From:  copyParticipantToProto(&copyEvent.From),
			To:    copyParticipantToProto(&copyEvent.To),
			RwIdx: uint64(copyEvent.RwIdx),
			Data:  copyEvent.Data,
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

var copyLocationToProtoMap = map[CopyLocation]pb.CopyLocation{
	CopyLocationMemory:     pb.CopyLocation_MEMORY,
	CopyLocationBytecode:   pb.CopyLocation_BYTECODE,
	CopyLocationCalldata:   pb.CopyLocation_CALLDATA,
	CopyLocationLog:        pb.CopyLocation_LOG,
	CopyLocationKeccak:     pb.CopyLocation_KECCAK,
	CopyLocationReturnData: pb.CopyLocation_RETURNDATA,
}

var protoCopyLocationMap = common.ReverseMap(copyLocationToProtoMap)

func copyParticipantFromProto(participant *pb.CopyParticipant) CopyParticipant {
	ret := CopyParticipant{
		Location:   protoCopyLocationMap[participant.Location],
		MemAddress: participant.MemAddress,
	}
	switch id := participant.GetId().(type) {
	case *pb.CopyParticipant_CallId:
		txId := uint(id.CallId)
		ret.TxId = &txId
	case *pb.CopyParticipant_BytecodeHash:
		hash := common.HexToHash(id.BytecodeHash)
		ret.BytecodeHash = &hash
	case *pb.CopyParticipant_KeccakHash:
		hash := common.HexToHash(id.KeccakHash)
		ret.KeccakHash = &hash
	}
	return ret
}

func copyParticipantToProto(participant *CopyParticipant) *pb.CopyParticipant {
	ret := &pb.CopyParticipant{
		Location:   copyLocationToProtoMap[participant.Location],
		MemAddress: participant.MemAddress,
	}
	switch {
	case participant.TxId != nil:
		ret.Id = &pb.CopyParticipant_CallId{CallId: uint64(*participant.TxId)}
	case participant.BytecodeHash != nil:
		ret.Id = &pb.CopyParticipant_BytecodeHash{BytecodeHash: participant.BytecodeHash.Hex()}
	case participant.KeccakHash != nil:
		ret.Id = &pb.CopyParticipant_KeccakHash{KeccakHash: participant.KeccakHash.Hex()}
	}
	return ret
}
